package wxapi

import "github.com/zeromicro/go-zero/core/stores/redis"

type Config struct {
	AppId       string          `json:"appId"`
	Secret      string          `json:"secret"`
	RedisConfig redis.RedisConf `json:"redisConfig,optional"`
}
type Subscribe struct {
	//模板id
	TemplateId string `json:"templateId"`
	//接收者 openId
	Touser string `json:"touser"`
	//模板内容 固定格式
	Data Data `json:"data"`
	//要跳转的页面
	Page string `json:"page"`
	//小程序跳转类型
	MiniprogramState string `json:"miniprogramState"`
	//语言
	Lang string `json:"lang"`
}
type DataItem struct {
	Value string `json:"value"`
}

type Data struct {
	Thing1           DataItem `json:"thing1,optional,omitempty"`
	Thing2           DataItem `json:"thing2,optional,omitempty"`
	Thing5           DataItem `json:"thing5,optional,omitempty"`
	Thing10          DataItem `json:"thing10,optional,omitempty"`
	Date4            DataItem `json:"date4,optional,omitempty"`
	Thing6           DataItem `json:"thing6,optional,omitempty"`
	Thing8           DataItem `json:"thing8,optional,omitempty"`
	Time3            DataItem `json:"time3,optional,omitempty"`
	Amount3          DataItem `json:"amount3,optional,omitempty"`
	Date5            DataItem `json:"date5,optional,omitempty"`
	Thing4           DataItem `json:"thing4,optional,omitempty"`
	CharacterString6 DataItem `json:"character_string6,optional,omitempty"`
}
