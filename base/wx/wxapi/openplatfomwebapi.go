// 文件用途：处理网页登录授权、应用 token、订阅发送和内容安全检测。

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

// OpenPlatformWebHelper 封装网页登录、订阅消息和内容安全检测，持有应用配置及 Redis 客户端。
type OpenPlatformWebHelper struct {
	RedisConn *redis.Redis // 复用的 Redis 客户端。
	C         Config       // 当前微信应用配置。
}

// NewOpenPlatformWebHelper 创建网页授权、订阅消息和内容检测辅助服务。
// Redis 配置无效时返回错误，应用凭证由 Config 注入。
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

// AccessTokenKey 是应用级 access_token 缓存 key 模板，%s 替换为 AppID。
// 此 key 不能用于保存用户级 OAuth 凭证。
const AccessTokenKey = "open_platform_access_token:%s"

// 获取访问令牌地址
const tokenURLFormat = "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s"

// GetOpenId 使用当前网页登录 code 直接换取该用户 OpenID。
// 不复用全局用户凭证，也不在 Redis 缓存 OAuth token，避免串用用户身份。
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

// SendSubscribe 获取应用级 token 后发送订阅消息，template 指定接收人及模板字段。
// 成功需微信 errcode=0；接收人订阅授权由业务流程负责。
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

// GetAccessToken 请求微信应用级 access_token，仅返回 token 文本，不负责缓存。
// 缓存由上层辅助服务维护，调用失败会返回经过脱敏的错误。
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

// CheckTextLegal 提交文本到微信内容安全接口。
// scene 为微信约定场景值；调用方还需检查结果中的 Suggest，errcode=0 不代表内容一定通过。
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

// CheckMediaLegal 提交图片、音视频等媒体地址进行异步安全检测。
// 返回成功仅表示受理，最终结果需由业务项目按微信回调协议处理。
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
