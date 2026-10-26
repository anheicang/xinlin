package wxapi

import (
	"encoding/json"
	"errors"
	"fmt"

	"net/http"
	"net/url"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type OpenPlatformWebHelper struct {
	RedisConn *redis.Redis
	C         Config
}

func NewOpenPlatformWebHelper(c Config) (*OpenPlatformWebHelper, error) {
	redisConn, err := redis.NewRedis(c.RedisConfig)
	if err != nil {
		return nil, err
	}
	return &OpenPlatformWebHelper{
		RedisConn: redisConn,
		C:         c,
	}, nil
}

const AccessTokenKey = "open_platform_access_token:%s"

// 获取访问令牌地址
const tokenURLFormat = "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s"

// GetOpenId exchanges the current authorization code for that user's identity.
// OAuth credentials are user-specific and must never use the application token cache.
func (o *OpenPlatformWebHelper) GetOpenId(code string) (string, error) {
	if o == nil || o.C.AppId == "" || o.C.Secret == "" || strings.TrimSpace(code) == "" {
		return "", errors.New("invalid OAuth configuration or code")
	}
	query := url.Values{
		"appid": {o.C.AppId}, "secret": {o.C.Secret},
		"code": {code}, "grant_type": {"authorization_code"},
	}
	client := newWXClient()
	resp, err := wxRequest(client.R(), http.MethodGet, "https://api.weixin.qq.com/sns/oauth2/access_token?"+query.Encode())
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode())
	}
	var result AccessTokenResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", wxDecodeError()
	}
	if result.ErrorCode != 0 {
		return "", wxResponseError(result.ErrorCode)
	}
	if result.OpenId == "" {
		return "", wxDecodeError()
	}
	return result.OpenId, nil
}

// 发送订阅消息
func (w *OpenPlatformWebHelper) SendSubscribe(config Config, template Subscribe) (*ErrorResponse, error) {
	accessToken, err := w.RedisConn.Get(fmt.Sprintf(AccessTokenKey, config.AppId))
	if accessToken == "" {
		token, err := GetAccessToken(config.AppId, config.Secret)
		if err != nil {
			return nil, err
		}
		accessToken = token
		err = w.RedisConn.Setex(fmt.Sprintf(AccessTokenKey, config.AppId), token, 7000)
		if err != nil {
			return nil, err
		}
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=%s", accessToken) // 准备请求的 payload
	payload := map[string]interface{}{
		"template_id":       template.TemplateId,
		"touser":            template.Touser,
		"data":              template.Data,
		"page":              template.Page,
		"miniprogram_state": template.MiniprogramState,
		"lang":              template.Lang,
	}
	// 创建 http POST 请求
	client := newWXClient()
	request := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload)
	resp, err := wxRequest(request, http.MethodPost, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("意外的 HTTP 状态码: %d", resp.StatusCode())
	}
	// 将 HTTP 请求的 body 数据解析到我们定义的结构体中
	wxResp := new(ErrorResponse)
	if err := json.Unmarshal(resp.Body(), wxResp); err != nil {
		return nil, wxDecodeError()
	}
	if wxResp.ErrorCode != 0 {
		return nil, wxResponseError(int64(wxResp.ErrorCode))
	}
	return wxResp, nil
}

// GetAccessToken 得到访问令牌
func GetAccessToken(appID, secret string) (string, error) {
	getTokenURL := fmt.Sprintf(tokenURLFormat, appID, secret)
	client := newWXClient()
	resp, err := wxRequest(client.R(), http.MethodGet, getTokenURL)
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != 200 {
		return "", ErrGetTokenFail
	}
	var result map[string]interface{}
	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return "", wxDecodeError()
	}
	// 获取访问 token 令牌
	token, ok := result["access_token"].(string)
	if !ok {
		return "", ErrGetTokenFail
	}
	return token, nil
}

// 微信文本安全内容检测方法
func (w *OpenPlatformWebHelper) CheckTextLegal(config Config, content, openId string, scene int64) (*CheckResult, error) {
	accessToken, err := w.RedisConn.Get(fmt.Sprintf(AccessTokenKey, config.AppId))
	if accessToken == "" {
		token, err := GetAccessToken(config.AppId, config.Secret)
		if err != nil {
			return nil, err
		}
		accessToken = token
		err = w.RedisConn.Setex(fmt.Sprintf(AccessTokenKey, config.AppId), token, 7000)
		if err != nil {
			return nil, err
		}
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/wxa/msg_sec_check?access_token=%s", accessToken) // 准备请求的 payload
	payload := map[string]interface{}{
		"content": content,
		"version": 2, //固定值
		"scene":   scene,
		"openid":  openId,
	}
	// 创建 http POST 请求
	client := newWXClient()
	request := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload)
	resp, err := wxRequest(request, http.MethodPost, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("意外的 HTTP 状态码: %d", resp.StatusCode())
	}
	// 将 HTTP 请求的 body 数据解析到我们定义的结构体中
	wxResp := new(CheckResult)
	if err := json.Unmarshal(resp.Body(), wxResp); err != nil {
		return nil, wxDecodeError()
	}
	if wxResp.ErrCode != 0 {
		return nil, wxResponseError(int64(wxResp.ErrCode))
	}
	return wxResp, nil
}

// CheckMediaLegal 微信音视频,图片检测方法
func (w *OpenPlatformWebHelper) CheckMediaLegal(config Config, mediaUrl, openId string, scene, mediaType int64) (*CheckResult, error) {
	accessToken, err := w.RedisConn.Get(fmt.Sprintf(AccessTokenKey, config.AppId))
	if accessToken == "" {
		token, err := GetAccessToken(config.AppId, config.Secret)
		if err != nil {
			return nil, err
		}
		accessToken = token
		err = w.RedisConn.Setex(fmt.Sprintf(AccessTokenKey, config.AppId), token, 7000)
		if err != nil {
			return nil, err
		}
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/wxa/media_check_async?access_token=%s", accessToken) // 准备请求的 payload
	payload := map[string]interface{}{
		"media_url":  mediaUrl,
		"version":    2, //固定值
		"scene":      scene,
		"media_type": mediaType,
		"openid":     openId,
	}
	// 创建 http POST 请求
	client := newWXClient()
	request := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload)
	resp, err := wxRequest(request, http.MethodPost, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("意外的 HTTP 状态码: %d", resp.StatusCode())
	}
	// 将 HTTP 请求的 body 数据解析到我们定义的结构体中
	wxResp := new(CheckResult)
	if err := json.Unmarshal(resp.Body(), wxResp); err != nil {
		return nil, wxDecodeError()
	}
	if wxResp.ErrCode != 0 {
		return nil, wxResponseError(int64(wxResp.ErrCode))
	}
	return wxResp, nil
}
