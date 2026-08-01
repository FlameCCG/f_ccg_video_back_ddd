// Package email 提供 SMTP 发信与邮箱地址校验适配器（原 service/email_service
// 与 api/captcha_api 中的包级 email-verifier 实例）。
//
// 本包实现 application/identity 声明的 EmailCaptchaSender 与 EmailAddressVerifier 端口；
// 邮件主题、正文模板、SMTP 握手/代理策略与原实现逐字一致。
package email

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	emaillib "github.com/jordan-wright/email"
	"go.uber.org/zap"
)

const (
	smtpDialTimeout    = 15 * time.Second
	smtpSessionTimeout = 30 * time.Second
)

// smtpEnvelope 组装好的 SMTP 信封（发件人、收件人与原始报文）。
type smtpEnvelope struct {
	sender     string
	recipients []string
	raw        []byte
}

// bufferedConn 让 CONNECT 阶段已缓冲的字节能继续被 SMTP 客户端读到。
type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

// Read 从缓冲读取器读取数据。
func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// Sender SMTP 邮件发送器，同时提供验证码邮件模板。
type Sender struct {
	cfgProvider *config.Provider
	logger      *zap.Logger
}

var _ app.EmailCaptchaSender = (*Sender)(nil)

// NewSender 构造 SMTP 邮件发送器。
func NewSender(cfgProvider *config.Provider, logger *zap.Logger) *Sender {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Sender{cfgProvider: cfgProvider, logger: logger}
}

// Send 按站点邮件配置发送一封 text + html 邮件。
func (s *Sender) Send(to, subject, textBody, htmlBody string) error {
	em, err := s.conf()
	if err != nil {
		return err
	}

	e := emaillib.NewEmail()
	e.From = fmt.Sprintf("%s <%s>", em.SendNickName, em.SendEmail)
	e.To = []string{to}
	e.Subject = subject
	e.Text = []byte(textBody)
	e.HTML = []byte(htmlBody)

	if err := sendSMTPMail(e, em.Domain, em.Port, em.SendEmail, em.AuthCode, em.TLS, em.SSL, s.logger); err != nil {
		s.logger.Error("send email error", zap.Error(err))

		return err
	}

	return nil
}

func (s *Sender) conf() (*config.Email, error) {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Email == nil {
		return nil, errors.New("email config is missing")
	}

	return cfg.Email, nil
}

func sendSMTPMail(
	e *emaillib.Email,
	domain string,
	port int,
	username, password string,
	enableStartTLS, enableSSL bool,
	logger *zap.Logger,
) error {
	envelope, err := buildSMTPEnvelope(e)
	if err != nil {
		return err
	}

	targetAddr := net.JoinHostPort(domain, strconv.Itoa(port))

	conn, err := dialSMTP(targetAddr, logger)
	if err != nil {
		return err
	}

	defer func() { _ = conn.Close() }()

	// 整会话 deadline，覆盖握手与读写，避免半开连接挂死。
	_ = conn.SetDeadline(time.Now().Add(smtpSessionTimeout))

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: domain,
	}

	implicitTLS := port == 465 || (enableSSL && !enableStartTLS)
	if implicitTLS {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(context.Background()); err != nil {
			return fmt.Errorf("smtp tls handshake failed: %w", err)
		}

		conn = tlsConn
		_ = conn.SetDeadline(time.Now().Add(smtpSessionTimeout))
	}

	client, err := smtp.NewClient(conn, domain)
	if err != nil {
		return fmt.Errorf("create smtp client failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp hello failed: %w", err)
	}

	if !implicitTLS && enableStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp server %s does not support STARTTLS", domain)
		}

		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls failed: %w", err)
		}

		_ = conn.SetDeadline(time.Now().Add(smtpSessionTimeout))
	}

	if username != "" && password != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return fmt.Errorf("smtp server %s does not support AUTH", domain)
		}

		if err := client.Auth(smtp.PlainAuth("", username, password, domain)); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}

	if err := client.Mail(envelope.sender); err != nil {
		return fmt.Errorf("smtp mail from failed: %w", err)
	}

	for _, recipient := range envelope.recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp rcpt to %s failed: %w", recipient, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data failed: %w", err)
	}

	if _, err := writer.Write(envelope.raw); err != nil {
		_ = writer.Close()

		return fmt.Errorf("smtp write body failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp finalize body failed: %w", err)
	}

	// 仅 QUIT 阶段忽略部分服务商截断响应；DATA 成功后邮件通常已入队。
	if err := client.Quit(); err != nil && !isSMTPQuitIgnorable(err) {
		return fmt.Errorf("smtp quit failed: %w", err)
	}

	return nil
}

// isSMTPQuitIgnorable 仅在 QUIT 阶段容忍 short response 类兼容性问题。
func isSMTPQuitIgnorable(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "short response")
}

func buildSMTPEnvelope(e *emaillib.Email) (*smtpEnvelope, error) {
	recipients := make([]string, 0, len(e.To)+len(e.Cc)+len(e.Bcc))
	for _, addr := range append(append(append([]string{}, e.To...), e.Cc...), e.Bcc...) {
		parsed, err := mail.ParseAddress(addr)
		if err != nil {
			return nil, err
		}

		recipients = append(recipients, parsed.Address)
	}

	if e.From == "" || len(recipients) == 0 {
		return nil, errors.New("must specify at least one From address and one To address")
	}

	sender, err := mail.ParseAddress(e.From)
	if err != nil {
		return nil, err
	}

	raw, err := e.Bytes()
	if err != nil {
		return nil, err
	}

	return &smtpEnvelope{
		sender:     sender.Address,
		recipients: recipients,
		raw:        raw,
	}, nil
}

func dialSMTP(targetAddr string, logger *zap.Logger) (net.Conn, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if proxyURL, ok := smtpProxyURLFromEnv(); ok {
		conn, err := dialSMTPViaHTTPProxy(proxyURL, targetAddr, smtpDialTimeout)
		if err == nil {
			return conn, nil
		}

		directConn, directErr := (&net.Dialer{Timeout: smtpDialTimeout}).Dial("tcp", targetAddr)
		if directErr == nil {
			logger.Warn("smtp proxy dial failed, fallback to direct", zap.String("proxy", proxyURL.Redacted()), zap.Error(err))

			return directConn, nil
		}

		return nil, fmt.Errorf("smtp proxy dial failed: %w; direct dial failed: %w", err, directErr)
	}

	return (&net.Dialer{Timeout: smtpDialTimeout}).Dial("tcp", targetAddr)
}

func smtpProxyURLFromEnv() (*url.URL, bool) {
	for _, key := range []string{"ALL_PROXY", "all_proxy", "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}

		proxyURL, err := url.Parse(raw)
		if err != nil || proxyURL.Host == "" {
			continue
		}

		switch proxyURL.Scheme {
		case "http", "https":
			return proxyURL, true
		}
	}

	return nil, false
}

func dialSMTPViaHTTPProxy(proxyURL *url.URL, targetAddr string, timeout time.Duration) (net.Conn, error) {
	proxyAddr := proxyURL.Host
	if proxyURL.Port() == "" {
		switch proxyURL.Scheme {
		case "https":
			proxyAddr = net.JoinHostPort(proxyURL.Hostname(), "443")
		default:
			proxyAddr = net.JoinHostPort(proxyURL.Hostname(), "80")
		}
	}

	conn, err := (&net.Dialer{Timeout: timeout}).Dial("tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	if proxyURL.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: proxyURL.Hostname(),
		})
		if err := tlsConn.HandshakeContext(context.Background()); err != nil {
			_ = conn.Close()

			return nil, fmt.Errorf("proxy tls handshake failed: %w", err)
		}

		conn = tlsConn
	}

	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Connection: Keep-Alive\r\n", targetAddr, targetAddr)

	if proxyURL.User != nil {
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		connectReq += "Proxy-Authorization: Basic " + token + "\r\n"
	}

	connectReq += "\r\n"
	if _, err := io.WriteString(conn, connectReq); err != nil {
		_ = conn.Close()

		return nil, fmt.Errorf("write proxy connect request failed: %w", err)
	}

	reader := bufio.NewReader(conn)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()

		return nil, fmt.Errorf("read proxy response failed: %w", err)
	}

	statusLine = strings.TrimSpace(statusLine)

	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		_ = conn.Close()

		return nil, fmt.Errorf("invalid proxy response: %s", statusLine)
	}

	statusCode, err := strconv.Atoi(parts[1])
	if err != nil {
		_ = conn.Close()

		return nil, fmt.Errorf("invalid proxy status code: %s", statusLine)
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			_ = conn.Close()

			return nil, fmt.Errorf("read proxy response header failed: %w", err)
		}

		if line == "\r\n" {
			break
		}
	}

	if statusCode != http200 {
		_ = conn.Close()

		return nil, fmt.Errorf("proxy connect failed: %s", statusLine)
	}

	return &bufferedConn{Conn: conn, reader: reader}, nil
}

// http200 代理 CONNECT 成功的状态码。
const http200 = 200
