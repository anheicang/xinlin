// 文件用途：定义微信应用配置、订阅消息参数和现有模板字段。

package wxapi

import "github.com/zeromicro/go-zero/core/stores/redis"

// Config 定义微信应用 ID、应用密钥及 token 缓存所用的 Redis 配置。
type Config struct {
	AppId       string          `json:"appId"`                // 微信应用 AppID。
	Secret      string          `json:"secret"`               // 微信应用密钥，禁止输出到日志或响应。
	RedisConfig redis.RedisConf `json:"redisConfig,optional"` // 应用 token 缓存所用的 Redis 连接配置。
}

// Subscribe 定义一条微信订阅消息的接收人、模板内容和跳转参数。
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

// DataItem 表示订阅模板中单个字段的 value 对象。
type DataItem struct {
	Value string `json:"value"` // 模板字段的显示值，内容格式需满足微信模板要求。
}

// Data 映射现有微信订阅模板字段，字段编号必须与选用模板一致。
// 不同模板需由业务项目按微信规范扩展，不能随意修改编号。
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
