package wxapi

type ErrorResponse struct {
	ErrorCode int64  `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
}

type (
	WxLoginResponse struct {
		OpenId  string `json:"openid"`
		UnionID string `json:"unionid,omitempty"`
		//SessionKey string `json:"sessionKey"`
		ErrorResponse
	}

	WxPhoneInfo struct {
		PhoneNumber     string `json:"phoneNumber"`
		PurePhoneNumber string `json:"purePhoneNumber"`
		Watermark       struct {
			Timestamp int64  `json:"timestamp"`
			AppID     string `json:"appid"`
		} `json:"watermark"`
	}

	WxResponse struct {
		ErrorResponse
		PhoneInfo WxPhoneInfo `json:"phone_info"`
	}
)

// 微信开放平台
type (
	AccessTokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		OpenId       string `json:"openid"`
		Scope        string `json:"scope"`
		UnionId      string `json:"unionid"`
		ErrorResponse
	}
)
type CheckResult struct {
	ErrCode int64   `json:"errcode"`
	ErrMsg  string  `json:"errmsg"`
	Result  *Result `json:"result"`
}

type Result struct {
	Suggest string `json:"suggest"`
	Label   int64  `json:"label"`
}
type GetUserInfoResp struct {
	Subscribe      int    `json:"subscribe"`
	OpenID         string `json:"openid"`
	Language       string `json:"language"`
	SubscribeTime  int    `json:"subscribe_time"`
	UnionID        string `json:"unionid"`
	Remark         string `json:"remark"`
	GroupID        int    `json:"groupid"`
	TagIDList      []int  `json:"tagid_list"`
	SubscribeScene string `json:"subscribe_scene"`
	QrScene        int    `json:"qr_scene"`
	QrSceneStr     string `json:"qr_scene_str"`
}
type GetFollowUserListResp struct {
	Total int64       `json:"total"`
	Count int64       `json:"count"`
	Data  *FollowData `json:"data"`
}
type FollowData struct {
	OpenId     []string `json:"openid"`
	NextOpenid string   `json:"next_openid"`
}
