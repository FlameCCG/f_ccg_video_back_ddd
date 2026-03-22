package identity

import (
	"context"
	"time"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// 邮箱验证码业务类型（与请求体 type 字段取值一致）。
const (
	// EmailCaptchaTypeRegister 注册。
	EmailCaptchaTypeRegister = identitydomain.EmailCaptchaRegister
	// EmailCaptchaTypeResetPassword 重置密码。
	EmailCaptchaTypeResetPassword = identitydomain.EmailCaptchaPasswordRecovery
	// EmailCaptchaTypeBindEmail 绑定邮箱。
	EmailCaptchaTypeBindEmail = identitydomain.EmailCaptchaBinding
)

// EmailCaptchaSendService 邮箱验证码发送用例服务。
//
// EmailAddressVerifier 与 EmailCaptchaSender 通过端口注入，测试可提供 fake 实现。
type EmailCaptchaSendService struct {
	users     identitydomain.UserAccountRepository
	captcha   *CaptchaService
	verifier  EmailAddressVerifier
	sender    EmailCaptchaSender
	generator EmailCaptchaGenerator
	limiter   RateLimiter
	tasks     *ctxutil.DetachedTaskGroup
	logger    *zap.Logger
}

// NewEmailCaptchaSendService 构造邮箱验证码发送服务。
func NewEmailCaptchaSendService(
	users identitydomain.UserAccountRepository,
	captcha *CaptchaService,
	verifier EmailAddressVerifier,
	sender EmailCaptchaSender,
	generator EmailCaptchaGenerator,
	limiter RateLimiter,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *EmailCaptchaSendService {
	return &EmailCaptchaSendService{
		users:     users,
		captcha:   captcha,
		verifier:  verifier,
		sender:    sender,
		generator: generator,
		limiter:   limiter,
		tasks:     tasks,
		logger:    logger,
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
	sending := (identitydomain.EmailCaptchaPolicy{}).Sending()
	id, code := s.generator.Generate(sending.CodeLength, sending.Charset)
	if id == "" || code == "" {
		return nil, ErrEmailCaptchaStoreFailed
	}

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
	sending := (identitydomain.EmailCaptchaPolicy{}).Sending()
	checks := []struct {
		subject SendLimitSubject
		limit   int64
	}{
		{subject: SendLimitSubject{Type: in.Type, Scope: SendLimitEmail, Value: in.Email}, limit: sending.PerEmail},
		{subject: SendLimitSubject{Type: in.Type, Scope: SendLimitIP, Value: in.ClientIP}, limit: sending.PerIP},
	}

	for _, check := range checks {
		ok, err := s.limiter.Allow(ctx, check.subject, check.limit, sending.Window)
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

	validationErr := (identitydomain.EmailAddressFacts{SyntaxValid: ret.SyntaxValid, HasSMTPResult: ret.HasSMTPResult, SMTPHostExists: ret.SMTPHostExists, HasMXRecords: ret.HasMXRecords}).EnsureReachable()
	if validationErr != nil {
		s.logger.Error(validationErr.Error())
	}
	return validationErr
}

// checkBusinessPrecondition 按业务类型校验邮箱占用状态。
//
// 查询报错时不额外区分——注册/绑定继续放行（唯一索引兜底），
// 重置密码则回「该邮箱未注册」。
func (s *EmailCaptchaSendService) checkBusinessPrecondition(ctx context.Context, in EmailCaptchaSendInput) error {
	user, err := s.users.FindByEmail(ctx, in.Email)

	qualificationErr := (identitydomain.EmailCaptchaPolicy{}).EnsureAccount(in.Type, user != nil, err == nil)
	if qualificationErr != nil {
		if qualificationErr == ErrEmailCaptchaEmailNotRegistered {
			s.logger.Error(qualificationErr.Error(), zap.Error(err))
		} else {
			s.logger.Error(qualificationErr.Error())
		}
	}
	return qualificationErr
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
