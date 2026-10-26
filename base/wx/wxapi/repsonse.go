// 文件用途：映射微信接口 JSON 响应，包括统一 errcode 与用户信息。

package wxapi

// ErrorResponse 映射微信通用 errcode/errmsg；errcode 为 0 时才表示业务成功。
type ErrorResponse struct {
	ErrorCode int64  `json:"errcode"` // 微信业务错误码，0 表示该接口没有报告业务错误。
	ErrMsg    string `json:"errmsg"`  // 微信原始错误描述，可能含敏感内容，不直接对外透传。
}

type (
	// WxLoginResponse 保存小程序登录返回的 OpenID/UnionID 和微信错误信息。
	WxLoginResponse struct {
		OpenId  string `json:"openid"`            // 用户在当前应用下的 OpenID。
		UnionID string `json:"unionid,omitempty"` // 同一微信开放平台下的用户关联标识，可能为空。
		//SessionKey string `json:"sessionKey"`
		ErrorResponse
	}

	// WxPhoneInfo 保存授权手机号、纯号码以及来源应用的水印信息。
	WxPhoneInfo struct {
		PhoneNumber     string   `json:"phoneNumber"`     // 微信返回的手机号，可能包含国家区号信息。
		PurePhoneNumber string   `json:"purePhoneNumber"` // 不含国家区号的手机号。
		Watermark       struct { // 微信附带的来源水印，包含应用 ID 与时间。
			Timestamp int64  `json:"timestamp"` // 来源水印时间戳，单位为秒。
			AppID     string `json:"appid"`     // 来源水印中声明的微信应用 ID。
		} `json:"watermark"`
	}

	// WxResponse 保存手机号换取结果及微信错误码。
	WxResponse struct {
		ErrorResponse
		PhoneInfo WxPhoneInfo `json:"phone_info"` // 用户授权后换取的手机号信息。
	}
)

// 微信开放平台
type (
	// AccessTokenResponse 保存网页 OAuth 的用户凭证、身份和授权范围，与应用级 token 区分使用。
	AccessTokenResponse struct {
		AccessToken  string `json:"access_token"`  // 用户 OAuth access_token，不应与应用级 token 混用。
		ExpiresIn    int64  `json:"expires_in"`    // 供应商声明的 token 有效期秒数。
		RefreshToken string `json:"refresh_token"` // 用户 OAuth 刷新凭证，禁止写入普通日志。
		OpenId       string `json:"openid"`        // 用户在当前应用下的 OpenID。
		Scope        string `json:"scope"`         // 用户授予的 OAuth 权限范围。
		UnionId      string `json:"unionid"`       // 同一微信开放平台下的用户关联标识。
		ErrorResponse
	}
)

// CheckResult 保存微信内容安全检测的受理/检测结果，异步检测需结合回调处理。
type CheckResult struct {
	ErrCode int64   `json:"errcode"` // 内容检测接口业务错误码。
	ErrMsg  string  `json:"errmsg"`  // 微信原始错误描述，可能含敏感内容，不直接对外透传。
	Result  *Result `json:"result"`  // 文本检测结果；异步媒体检测时可能为空。
}

// Result 保存文本检测建议和标签；Suggest 可能为 pass、review 或 risky。
type Result struct {
	Suggest string `json:"suggest"` // 检测建议，例如 pass、review 或 risky。
	Label   int64  `json:"label"`   // 微信内容检测分类标签。
}

// GetUserInfoResp 保存公众号用户基本信息及微信业务错误。
type GetUserInfoResp struct {
	ErrorResponse
	Subscribe      int    `json:"subscribe"`       // 是否关注公众号，按微信接口约定取值。
	OpenID         string `json:"openid"`          // 公众号用户 OpenID。
	Language       string `json:"language"`        // 用户语言标识。
	SubscribeTime  int    `json:"subscribe_time"`  // 关注时间的 Unix 秒时间戳。
	UnionID        string `json:"unionid"`         // 同一微信开放平台下的用户关联标识，可能为空。
	Remark         string `json:"remark"`          // 公众号对用户设置的备注。
	GroupID        int    `json:"groupid"`         // 微信旧用户分组 ID。
	TagIDList      []int  `json:"tagid_list"`      // 公众号用户标签 ID 列表。
	SubscribeScene string `json:"subscribe_scene"` // 用户关注渠道。
	QrScene        int    `json:"qr_scene"`        // 数值形式的二维码场景值。
	QrSceneStr     string `json:"qr_scene_str"`    // 字符串形式的二维码场景值。
}

// GetFollowUserListResp 保存公众号关注列表一页结果及总数、数量和业务错误。
type GetFollowUserListResp struct {
	ErrorResponse
	Total int64       `json:"total"` // 总记录数，不一定等于当前页条数。
	Count int64       `json:"count"` // 当前页返回的用户数量。
	Data  *FollowData `json:"data"`  // 当前页关注用户数据。
}

// FollowData 保存一页 OpenID 集合；NextOpenid 字段沿用旧结构映射，当前查询函数不自动翻页。
type FollowData struct {
	OpenId     []string `json:"openid"`      // 当前页用户 OpenID 列表。
	NextOpenid string   `json:"next_openid"` // 旧结构中的下一页游标字段；本包查询入口目前不自动分页。
}
