package wxapi

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/threading"
	"net/http"
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

func (o *OpenPlatformWebHelper) refreshAccessTokenAndGetOpenId(refreshToken string) (string, error) {
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/refresh_token?appid=%s&grant_type=refresh_token&refresh_token=%s", o.C.AppId, refreshToken)
	client := resty.New()
	resp, err := wxRequest(client.R(), http.MethodGet, url)
	if err != nil {
		return "", err
	}
	wxResp := new(AccessTokenResponse)
	if err = json.Unmarshal(resp.Body(), wxResp); err != nil {
		return "", wxDecodeError()
	}

	if wxResp.ErrorCode != 0 {
		return "", wxResponseError(wxResp.ErrorCode)
	}

	return wxResp.OpenId, o.RedisConn.Setex("open_platform_web_access_token", wxResp.AccessToken, 3600*24*28)
}

func (o *OpenPlatformWebHelper) GetOpenId(code string) (string, error) {
	openId := ""
	accessToken, err := o.RedisConn.Get("open_platform_web_access_token")
	if err != nil {
		return "", err
	}
	if accessToken == "" {
		url := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code", o.C.AppId, o.C.Secret, code)

		// 创建http get请求
		client := resty.New()
		resp, err := wxRequest(client.R(), http.MethodGet, url)
		if err != nil {
			return "", err
		}
		wxResp := new(AccessTokenResponse)
		if err = json.Unmarshal(resp.Body(), wxResp); err != nil {
			return "", wxDecodeError()
		}

		if wxResp.ErrorCode != 0 {
			return "", wxResponseError(wxResp.ErrorCode)
		}

		threading.GoSafe(func() {
			if _, err := o.refreshAccessTokenAndGetOpenId(wxResp.RefreshToken); err != nil {
				logx.Errorf("refreshAccessToken error: %v", err)
			}
		})

		openId = wxResp.OpenId
	}
	//else {
	//	openId, err = o.refreshAccessTokenAndGetOpenId(accessToken)
	//	if err != nil {
	//		return openId, err
	//	}
	//}

	return openId, nil
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
	client := resty.New()
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
	client := resty.New()
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
	client := resty.New()
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
	client := resty.New()
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
