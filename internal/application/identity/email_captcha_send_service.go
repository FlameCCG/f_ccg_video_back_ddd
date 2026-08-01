package identity

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/mojocn/base64Captcha"
	"go.uber.org/zap"
)

// 邮箱验证码业务类型（与请求体 type 字段取值一致）。
const (
	// EmailCaptchaTypeRegister 注册。
	EmailCaptchaTypeRegister = 1
	// EmailCaptchaTypeResetPassword 重置密码。
	EmailCaptchaTypeResetPassword = 2
	// EmailCaptchaTypeBindEmail 绑定邮箱。
	EmailCaptchaTypeBindEmail = 3
)

const (
	// emailCaptchaCodeLength 验证码长度。
	emailCaptchaCodeLength = 5
	// emailCaptchaCodeCharset 验证码字符集（与原实现逐字一致）。
	emailCaptchaCodeCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// emailCaptchaSendWindow 发送限流窗口。
	emailCaptchaSendWindow = 10 * time.Minute
	// emailCaptchaSendLimitPerEmail 单邮箱窗口内发送上限。
	emailCaptchaSendLimitPerEmail int64 = 5
	// emailCaptchaSendLimitPerIP 单 IP 窗口内发送上限。
	emailCaptchaSendLimitPerIP int64 = 20
)

// EmailCaptchaSendService 邮箱验证码发送用例服务（原 api/captcha_api/email_captcha.go）。
//
// 原实现用包级 emailverifier 实例与包级函数变量 emailCaptchaSend；
// 新架构改为注入 EmailAddressVerifier / EmailCaptchaSender 端口，测试注入 fake。
type EmailCaptchaSendService struct {
	users    domain.UserAccountRepository
	captcha  *CaptchaService
	verifier EmailAddressVerifier
	sender   EmailCaptchaSender
	limiter  RateLimiter
	tasks    *ctxutil.DetachedTaskGroup
	logger   *zap.Logger
}

// NewEmailCaptchaSendService 构造邮箱验证码发送服务。
func NewEmailCaptchaSendService(
	users domain.UserAccountRepository,
	captcha *CaptchaService,
	verifier EmailAddressVerifier,
	sender EmailCaptchaSender,
	limiter RateLimiter,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *EmailCaptchaSendService {
	return &EmailCaptchaSendService{
		users:    users,
		captcha:  captcha,
		verifier: verifier,
		sender:   sender,
		limiter:  limiter,
		tasks:    tasks,
		logger:   logger,
	}
}

// Send 校验频率与邮箱可达性、做业务前置检查，然后生成验证码并异步投递。
func (s *EmailCaptchaSendService) Send(ctx context.Context, in EmailCaptchaSendInput) (*EmailCaptchaSendResult, error) {
	startAt := time.Now()

	if ok, err := s.allowSend(ctx, in); err != nil || !ok {
		if err != nil {
			s.logger.Warn("email captcha send rate limit failed", zap.Error(err))
		}

		return nil, ErrEmailCaptchaRateLimited
	}

	if err := s.verifyEmailAddress(in.Email); err != nil {
		return nil, err
	}

	if err := s.checkBusinessPrecondition(ctx, in); err != nil {
		return nil, err
	}

	// 生成验证码（答案仅存验证码存储，禁止打印到日志/控制台）。
	code := base64Captcha.RandText(emailCaptchaCodeLength, emailCaptchaCodeCharset)
	id := base64Captcha.RandomId()

	if err := s.captcha.StoreEmailCaptcha(ctx, id, in.Email, code); err != nil {
		s.logger.Error("验证码保存失败", zap.Int("type", in.Type), zap.String("emailID", id), zap.Error(err))

		return nil, ErrEmailCaptchaStoreFailed
	}

	s.queueDelivery(ctx, in.Type, in.Email, id, code)

	cost := time.Since(startAt)

	s.logger.Info("邮箱验证码异步发送已提交",
		zap.Int("type", in.Type),
		zap.String("email", in.Email),
		zap.String("emailID", id),
		zap.Duration("cost", cost),
	)

	return &EmailCaptchaSendResult{EmailID: id, Cost: cost}, nil
}

// allowSend 按「邮箱 + 类型」与「IP + 类型」两个维度做固定窗口限流。
func (s *EmailCaptchaSendService) allowSend(ctx context.Context, in EmailCaptchaSendInput) (bool, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))

	checks := []struct {
		key   string
		limit int64
	}{
		{
			key:   fmt.Sprintf("%semail:%d:%s", constants.EmailCaptchaSendRateKeyPrefix, in.Type, email),
			limit: emailCaptchaSendLimitPerEmail,
		},
		{
			key:   fmt.Sprintf("%sip:%d:%s", constants.EmailCaptchaSendRateKeyPrefix, in.Type, strings.TrimSpace(in.ClientIP)),
			limit: emailCaptchaSendLimitPerIP,
		},
	}
	for _, check := range checks {
		ok, err := s.limiter.Allow(ctx, check.key, check.limit, emailCaptchaSendWindow)
		if err != nil || !ok {
			return ok, err
		}
	}

	return true, nil
}

// verifyEmailAddress 校验邮箱语法、SMTP 主机与 MX 记录。
func (s *EmailCaptchaSendService) verifyEmailAddress(email string) error {
	ret, err := s.verifier.Verify(email)
	if err != nil {
		s.logger.Error("验证邮箱失败", zap.Error(err))

		return ErrEmailVerifyFailed
	}

	if !ret.SyntaxValid {
		s.logger.Error("邮箱格式错误")

		return ErrEmailSyntaxInvalid
	}

	if ret.HasSMTPResult && !ret.SMTPHostExists {
		s.logger.Error("邮箱服务器不存在")

		return ErrEmailHostNotExists
	}

	if !ret.HasMXRecords {
		s.logger.Error("邮箱服务器无MX记录")

		return ErrEmailNoMXRecords
	}

	return nil
}

// checkBusinessPrecondition 按业务类型校验邮箱占用状态。
//
// 与原实现一致：查询报错时不额外区分——注册/绑定继续放行（唯一索引兜底），
// 重置密码则回「该邮箱未注册」。
func (s *EmailCaptchaSendService) checkBusinessPrecondition(ctx context.Context, in EmailCaptchaSendInput) error {
	user, err := s.users.FindByEmail(ctx, in.Email)

	switch in.Type {
	case EmailCaptchaTypeRegister:
		if err == nil && user != nil {
			s.logger.Error("该邮箱已注册")

			return ErrEmailAlreadyRegistered
		}
	case EmailCaptchaTypeResetPassword:
		if err != nil || user == nil {
			s.logger.Error("该邮箱未注册", zap.Error(err))

			return ErrEmailCaptchaEmailNotRegistered
		}
	case EmailCaptchaTypeBindEmail:
		if err == nil && user != nil {
			s.logger.Error("该邮箱已被绑定")

			return ErrEmailAlreadyBound
		}
	}

	return nil
}

// queueDelivery 把邮件投递放到 detached 任务中，接口立即返回。
func (s *EmailCaptchaSendService) queueDelivery(ctx context.Context, captchaType int, email, id, code string) {
	if s.tasks == nil {
		return
	}

	s.tasks.Go(ctx, ctxutil.AsyncLongTimeout, func(_ context.Context) {
		if err := s.sender.SendEmailCaptcha(captchaType, email, code); err != nil {
			s.logger.Warn("异步发送邮箱验证码失败",
				zap.Int("type", captchaType),
				zap.String("email", email),
				zap.String("emailID", id),
				zap.Error(err),
			)
		}
	})
}
