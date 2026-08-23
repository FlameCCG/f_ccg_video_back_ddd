package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	app "f_ccg_video_back_ddd/internal/application/identity"
	third_login "f_ccg_video_back_ddd/internal/config/thirdlogin"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/ctxutil"
)

const (
	qqTokenURL    = "https://graph.qq.com/oauth2.0/token"
	qqUserInfoURL = "https://graph.qq.com/user/get_user_info"
)

// QQAccessToken 表示 QQ OAuth access token 响应。
type QQAccessToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    string `json:"expires_in"`
	Openid       string `json:"openid"`
	RefreshToken string `json:"refresh_token"`
}

// UserQQInfo 表示 QQ 用户资料。
type UserQQInfo struct {
	Ret             int    `json:"ret"`                // 返回码，0表示成功，非0表示失败
	Msg             string `json:"msg"`                // 返回信息，成功时为"success"，失败时为错误信息
	IsLost          int    `json:"is_lost"`            // 是否丢失数据，0表示数据完整，1表示数据不完整
	Nickname        string `json:"nickname"`           // 用户昵称
	Gender          string `json:"gender"`             // 性别文本（如："男"、"女"）
	GenderType      int    `json:"gender_type"`        // 性别类型（0未知，1男，2女）
	Province        string `json:"province"`           // 省份
	City            string `json:"city"`               // 城市
	Year            string `json:"year"`               // 出生年份
	Figureurl       string `json:"figureurl"`          // 30x30像素的QQ头像URL
	Figureurl1      string `json:"figureurl_1"`        // 50x50像素的QQ头像URL
	Figureurl2      string `json:"figureurl_2"`        // 100x100像素的QQ头像URL
	FigureurlQq1    string `json:"figureurl_qq_1"`     // 40x40像素的QQ头像URL（可能带特效）
	FigureurlQq2    string `json:"figureurl_qq_2"`     // 100x100像素的QQ头像URL（可能带特效）
	FigureurlQq     string `json:"figureurl_qq"`       // 高清QQ头像URL（尺寸可能较大）
	IsYellowVip     string `json:"is_yellow_vip"`      // 是否是黄钻用户（"0"否，"1"是）
	Vip             string `json:"vip"`                // 是否是VIP用户（"0"否，"1"是）
	YellowVipLevel  string `json:"yellow_vip_level"`   // 黄钻等级（如："3"）
	Level           string `json:"level"`              // QQ等级（如："64"）
	IsYellowYearVip string `json:"is_yellow_year_vip"` // 是否是年费黄钻用户（"0"否，"1"是）
	Openid          string `json:"openid"`             // 用户的openid
}

// QQClient QQ OAuth 客户端。
type QQClient struct {
	cfgProvider *infraconfig.Provider
	httpClient  *http.Client
}

var _ app.QQOAuthClient = (*QQClient)(nil)

// NewQQClient 构造 QQ OAuth 客户端。
func NewQQClient(cfgProvider *infraconfig.Provider) *QQClient {
	return &QQClient{
		cfgProvider: cfgProvider,
		httpClient:  &http.Client{Timeout: ctxutil.RequestOAuthTimeout},
	}
}

// FetchUser 使用授权码换取 QQ 用户信息，并归一化为通用画像。
func (c *QQClient) FetchUser(ctx context.Context, code string) (*app.OAuthUserInfo, error) {
	info, err := c.GetUserInfo(ctx, code)
	if err != nil {
		return nil, err
	}

	return &app.OAuthUserInfo{
		OpenID: info.Openid,
		Avatar: info.FigureurlQq,
	}, nil
}

// GetUserInfo 使用授权码换取 QQ 用户信息。
func (c *QQClient) GetUserInfo(ctx context.Context, code string) (*UserQQInfo, error) {
	at, err := c.accessToken(ctx, code)
	if err != nil {
		return nil, err
	}

	qqConf, err := c.conf()
	if err != nil {
		return nil, err
	}

	p := url.Values{}
	p.Add("access_token", at.AccessToken)
	p.Add("oauth_consumer_key", qqConf.AppID)
	p.Add("openid", at.Openid)
	urlStr := qqUserInfoURL + "?" + p.Encode()

	body, err := c.get(ctx, urlStr, "qq user info failed")
	if err != nil {
		return nil, err
	}

	var qqInfo UserQQInfo
	if err = json.Unmarshal(body, &qqInfo); err != nil {
		return nil, err
	}

	qqInfo.Openid = at.Openid

	return &qqInfo, nil
}

func (c *QQClient) accessToken(ctx context.Context, code string) (*QQAccessToken, error) {
	qqConf, err := c.conf()
	if err != nil {
		return nil, err
	}

	p := url.Values{}
	p.Add("grant_type", "authorization_code")
	p.Add("client_id", qqConf.AppID)
	p.Add("client_secret", qqConf.AppKey)
	p.Add("code", code)
	p.Add("fmt", "json")
	p.Add("redirect_uri", qqConf.Redirect)
	p.Add("need_openid", "1")
	urlStr := qqTokenURL + "?" + p.Encode()

	body, err := c.get(ctx, urlStr, "qq access token failed")
	if err != nil {
		return nil, err
	}

	var accessToken QQAccessToken
	if err = json.Unmarshal(body, &accessToken); err != nil {
		return nil, err
	}

	return &accessToken, nil
}

// get 发起 GET 请求并在 4xx/5xx 时返回带状态文本的错误（与原实现一致）。
func (c *QQClient) get(ctx context.Context, urlStr, failMessage string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("%s: %s", failMessage, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func (c *QQClient) conf() (*third_login.QQ, error) {
	cfg := c.cfgProvider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.QQ == nil {
		return nil, errors.New("qq auth config is missing")
	}

	return cfg.ThirdLogin.QQ, nil
}
