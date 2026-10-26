package wxapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type WxHelper struct {
	RedisConn *redis.Redis
}

func NewWxHelper(redisConfig redis.RedisConf) *WxHelper {
	redisConn, err := redis.NewRedis(redisConfig)
	if err != nil {
		return nil
	}
	return &WxHelper{
		RedisConn: redisConn,
	}
}

func (w *WxHelper) Login(code string, config Config) (*WxLoginResponse, error) {
	url := "https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code"
	// 合成url, 这里的appId和secret是在微信公众平台上获取的
	url = fmt.Sprintf(url, config.AppId, config.Secret, code)

	// 创建http get请求
	client := newWXClient()
	resp, err := wxRequest(client.R(), http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode())
	}

	// 解析http请求中body 数据到我们定义的结构体中
	wxResp := new(WxLoginResponse)
	if err := json.Unmarshal(resp.Body(), wxResp); err != nil {
		return nil, wxDecodeError()
	}

	if wxResp.ErrorCode != 0 {
		return nil, wxResponseError(wxResp.ErrorCode)
	}

	return wxResp, err
}

func (w *WxHelper) handleAccessToken(appId, secret string) (string, error) {
	token, err := GetAccessToken(appId, secret)
	if err != nil {
		logx.Errorf("handleAccessToken GetAccessToken error appId=%s err=%v", appId, err)
		return "", err
	}

	err = w.RedisConn.Setex(fmt.Sprintf(AccessTokenKey, appId), token, 7000)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (w *WxHelper) GetPhoneData(code string, config Config) (*WxResponse, error) {
	accessToken, err := w.RedisConn.Get(fmt.Sprintf(AccessTokenKey, config.AppId))
	if accessToken == "" {
		accessToken, err = w.handleAccessToken(config.AppId, config.Secret)
		if err != nil {
			return nil, err
		}
	}
	requestPhone := func(token string) (*WxResponse, error) {
		url := fmt.Sprintf("https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=%s", token)
		request := newWXClient().R().
			SetHeader("Content-Type", "application/json").
			SetBody(map[string]interface{}{"code": code})
		resp, requestErr := wxRequest(request, http.MethodPost, url)
		if requestErr != nil {
			return nil, requestErr
		}
		if resp.StatusCode() != 200 {
			return nil, fmt.Errorf("意外的 HTTP 状态码: %d", resp.StatusCode())
		}
		wxResp := new(WxResponse)
		if unmarshalErr := json.Unmarshal(resp.Body(), wxResp); unmarshalErr != nil {
			return nil, wxDecodeError()
		}
		return wxResp, nil
	}

	wxResp, err := requestPhone(accessToken)
	if err != nil {
		return nil, err
	}
	if wxResp.ErrorCode == 40001 || wxResp.ErrorCode == 42001 {
		accessToken, err = w.handleAccessToken(config.AppId, config.Secret)
		if err != nil {
			logx.Errorf("GetPhoneData handleAccessToken error appId=%s err=%v", config.AppId, err)
			return nil, err
		}
		wxResp, err = requestPhone(accessToken)
		if err != nil {
			return nil, err
		}
	}
	if wxResp.ErrorCode != 0 {
		return nil, wxResponseError(wxResp.ErrorCode)
	}
	return wxResp, nil
}

const Get_User_Info = "https://api.weixin.qq.com/cgi-bin/user/info?access_token=%s&openid=%s&lang=zh_CN"
const Get_Follow_List = "https://api.weixin.qq.com/cgi-bin/user/get?access_token=%s&next_openid=%s"

// GetUserInfo 公众号获取用户基本信息
func GetUserInfo(openid, token string) (*GetUserInfoResp, error) {
	respInfo := new(GetUserInfoResp)
	getTokenURL := fmt.Sprintf(Get_User_Info, token, openid)
	client := newWXClient()
	resp, err := wxRequest(client.R(), http.MethodGet, getTokenURL)
	if err != nil {
		return respInfo, err
	}
	if resp.StatusCode() != 200 {
		return respInfo, ErrGetUserInfoFail
	}
	err = json.Unmarshal(resp.Body(), &respInfo)
	if err != nil {
		return respInfo, wxDecodeError()
	}
	if respInfo.ErrorCode != 0 {
		return nil, wxResponseError(respInfo.ErrorCode)
	}
	return respInfo, nil
}

// GetFollowUserList 公众号获取用户关注列表
func GetFollowUserList(token string) (*GetFollowUserListResp, error) {
	respInfo := new(GetFollowUserListResp)
	getTokenURL := fmt.Sprintf(Get_Follow_List, token, "")
	client := newWXClient()
	resp, err := wxRequest(client.R(), http.MethodGet, getTokenURL)
	if err != nil {
		return respInfo, err
	}
	if resp.StatusCode() != 200 {
		return respInfo, ErrGetUserInfoFail
	}
	err = json.Unmarshal(resp.Body(), &respInfo)
	if err != nil {
		return respInfo, wxDecodeError()
	}
	if respInfo.ErrorCode != 0 {
		return nil, wxResponseError(respInfo.ErrorCode)
	}
	return respInfo, nil
}
