package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/identity"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	persistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// fakeEmailCaptchaSender 替代真实 SMTP 投递。
//
// 原测试通过替换包级函数变量 emailCaptchaSend 打桩；
// 新架构里投递是注入的 app.EmailCaptchaSender 端口，直接注入 fake。
type fakeEmailCaptchaSender struct {
	send func(captchaType int, email, code string) error
}

// SendEmailCaptcha 转发到测试提供的函数。
func (f fakeEmailCaptchaSender) SendEmailCaptcha(captchaType int, email, code string) error {
	if f.send == nil {
		return nil
	}

	return f.send(captchaType, email, code)
}

// fakeEmailAddressVerifier 让邮箱可达性校验一律通过。
type fakeEmailAddressVerifier struct{}

// Verify 返回「语法合法、有 MX 记录」的结果。
func (fakeEmailAddressVerifier) Verify(_ string) (*app.EmailVerifyResult, error) {
	return &app.EmailVerifyResult{SyntaxValid: true, HasMXRecords: true}, nil
}

// emailCaptchaFixture 汇总邮箱验证码用例的处理器与探针。
type emailCaptchaFixture struct {
	handler *CaptchaHandler
	tasks   *ctxutil.DetachedTaskGroup
	redis   *redis.Client
}

// newEmailCaptchaFixture 装配邮箱验证码发送用例（miniredis + sqlite，全离线）。
func newEmailCaptchaFixture(t *testing.T, sender app.EmailCaptchaSender) *emailCaptchaFixture {
	t.Helper()

	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = redisClient.Close() })

	db := openTestDB(t, &po.User{}, &po.UserConf{}, &po.FavoriteFolder{})
	provider := infraconfig.NewProvider(&config.Config{
		Email: &config.Email{IsExpire: 5},
	})

	captchaSvc := app.NewCaptchaService(nil, nil, nil, captcha.NewEmailCaptchaStore(redisClient), provider)
	tasks := ctxutil.NewDetachedTaskGroup()

	t.Cleanup(func() { _ = tasks.Stop(context.Background()) })

	emails := app.NewEmailCaptchaSendService(
		persistence.NewUserRepository(db, provider),
		captchaSvc,
		fakeEmailAddressVerifier{},
		sender,
		captcha.NewEmailGenerator(),
		// 用真实限流器 + miniredis：与生产路径一致（限流端口不可为 nil）。
		ratelimit.New(redisClient),
		tasks,
		zap.NewNop(),
	)

	return &emailCaptchaFixture{
		handler: NewCaptchaHandler(captchaSvc, emails, zap.NewNop()),
		tasks:   tasks,
		redis:   redisClient,
	}
}

// sendEmailCaptcha 调用 EmailCaptchaView 并返回响应体。
func (f *emailCaptchaFixture) sendEmailCaptcha(t *testing.T, email string) res.Response {
	t.Helper()

	recorder, ctx := newTestContext(http.MethodPost, "/v1/common/captcha/email")
	ctx.Set("request", EmailCaptchaRequest{
		Type:  app.EmailCaptchaTypeRegister,
		Email: email,
	})

	f.handler.EmailCaptchaView(ctx)

	return decodeResponse(t, recorder)
}

// TestEmailCaptchaViewReturnsImmediately 验证邮件投递是异步的：
// 即使发送方阻塞，接口也立即返回（原 TestQueueEmailCaptchaDeliveryReturnsImmediately）。
func TestEmailCaptchaViewReturnsImmediately(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	release := make(chan struct{})

	fixture := newEmailCaptchaFixture(t, fakeEmailCaptchaSender{
		send: func(_ int, _, _ string) error {
			started <- struct{}{}

			<-release

			return nil
		},
	})

	t.Cleanup(func() { close(release) })

	startAt := time.Now()
	response := fixture.sendEmailCaptcha(t, "slow@example.com")
	elapsed := time.Since(startAt)

	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

	require.Equal(t, "验证码发送中，请稍候查收", response.Msg, "unexpected response msg: %s", response.Msg)

	// 阻塞的发送方不得拖慢接口：留出宽松阈值以容忍 sqlite/miniredis 的固定开销。
	require.LessOrEqual(t, elapsed, time.Second, "expected handler to return without waiting for delivery, got %s", elapsed)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected background email sender to start")
	}
}

// TestEmailCaptchaViewIgnoresSendFailure 验证异步发送失败不影响接口成功响应
// （原 TestQueueEmailCaptchaDeliveryIgnoresSendFailure）。
func TestEmailCaptchaViewIgnoresSendFailure(t *testing.T) {
	t.Parallel()

	done := make(chan struct{}, 1)

	fixture := newEmailCaptchaFixture(t, fakeEmailCaptchaSender{
		send: func(_ int, _, _ string) error {
			done <- struct{}{}

			return context.DeadlineExceeded
		},
	})

	response := fixture.sendEmailCaptcha(t, "fail@example.com")
	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected failed async sender to still run")
	}
}

// TestEmailCaptchaViewStoresConsumableCaptcha 验证验证码已写入存储且可被中间件消费一次。
func TestEmailCaptchaViewStoresConsumableCaptcha(t *testing.T) {
	t.Parallel()

	codes := make(chan string, 1)

	fixture := newEmailCaptchaFixture(t, fakeEmailCaptchaSender{
		send: func(_ int, _, code string) error {
			codes <- code

			return nil
		},
	})

	response := fixture.sendEmailCaptcha(t, "store@example.com")
	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

	var payload EmailCaptchaResponse
	decodeResponseData(t, response, &payload)

	require.NotEmpty(t, payload.EmailID, "expected non-empty emailID")

	var sentCode string
	select {
	case sentCode = <-codes:
	case <-time.After(time.Second):
		t.Fatal("expected async sender to receive the code")
	}

	stored, err := app.NewCaptchaService(nil, nil, nil,
		captcha.NewEmailCaptchaStore(fixture.redis), infraconfig.NewProvider(&config.Config{}),
	).ConsumeEmailCaptcha(context.Background(), payload.EmailID)
	require.NoError(t, err, "consume stored captcha failed: %v", err)

	require.Equal(t, "store@example.com", stored.Email, "unexpected stored email: %s", stored.Email)

	require.Equal(t, stored.Code, sentCode, "stored code %q != sent code %q", stored.Code, sentCode)
}
