// 文件用途：定义快递查询请求、轨迹和响应结构，以及成功/签收状态判断。

package kd100

// QueryRequest 定义快递100实时查询参数，JSON tag 遵循供应商字段名。
type QueryRequest struct {
	Company  string `json:"com"`             // 快递公司编码，对应供应商 com 参数。
	Number   string `json:"num"`             // 快递运单号。
	Phone    string `json:"phone,omitempty"` // 部分承运商要求的手机号，可为空。
	To       string `json:"to,omitempty"`    // 目的地信息，可为空。
	ResultV2 string `json:"resultv2"`        // 供应商结果格式版本，构造函数默认为 4。
}

// NewQueryRequest 构造快递查询参数，默认使用 resultv2=4 返回扩展轨迹信息。
// phone 为供应商查询可能要求的手机号，允许为空。
func NewQueryRequest(company, number, phone string) *QueryRequest {
	return &QueryRequest{Company: company, Number: number, Phone: phone, ResultV2: "4"}
}

// TrackData 表示单条物流轨迹，包含时间、描述、位置和状态信息。
type TrackData struct {
	Time       string `json:"time"`       // 上游原始轨迹时间文本。
	Ftime      string `json:"ftime"`      // 格式化轨迹时间，供本包按上海时区解析。
	Context    string `json:"context"`    // 本条轨迹的事件描述。
	Location   string `json:"location"`   // 上游返回的位置描述。
	Status     string `json:"status"`     // 本条轨迹状态码。
	AreaCode   string `json:"areaCode"`   // 行政区域编码。
	AreaName   string `json:"areaName"`   // 行政区域名称。
	AreaCenter string `json:"areaCenter"` // 区域中心坐标文本。
	AreaPinYin string `json:"areaPinYin"` // 区域名称拼音。
}

// QueryResponse 映射快递100不同版本的查询结果，业务成功需调用 IsSuccess 判断。
type QueryResponse struct {
	Message    string      `json:"message"`    // 供应商查询描述，成功判断要求为 ok。
	State      string      `json:"state"`      // 物流状态码，例如 3 或 204 表示已签收。
	Status     string      `json:"status"`     // 旧响应格式的业务状态码。
	ReturnCode string      `json:"returnCode"` // 新响应格式的业务码，APIStatus 优先读取它。
	Condition  string      `json:"condition"`  // 供应商提供的运单状态描述。
	IsCheck    string      `json:"ischeck"`    // 签收标记，字符串 1 表示已签收。
	Com        string      `json:"com"`        // 响应中的快递公司编码。
	Nu         string      `json:"nu"`         // 响应中的运单号。
	Data       []TrackData `json:"data"`       // 物流轨迹条目。
	RouteInfo  RouteInfo   `json:"routeInfo"`  // 起点、当前位置和终点路由信息。
	ReturnFlag string      `json:"returnFlag"` // 供应商返回标记，具体含义以接口文档为准。
	ComOld     string      `json:"comOld"`     // 供应商返回的原快递公司编码。
	BrandInfo  BrandInfo   `json:"brandInfo"`  // 快递品牌与客服电话信息。
	LastResult LastResult  `json:"lastResult"` // 上游最近查询结果摘要。
}

// APIStatus 返回供应商业务状态，优先使用 returnCode，缺失时回退到 status。
// nil 响应返回空字符串，兼容快递100不同响应格式。
func (r *QueryResponse) APIStatus() string {
	if r == nil {
		return ""
	}
	if r.ReturnCode != "" {
		return r.ReturnCode
	}
	return r.Status
}

// IsSuccess 判断响应是否同时满足 message=ok 和业务状态 200。
func (r *QueryResponse) IsSuccess() bool {
	return r != nil && r.Message == "ok" && r.APIStatus() == "200"
}

// IsSigned 根据 ischeck 或物流状态码判断是否已签收；nil 响应返回 false。
func (r *QueryResponse) IsSigned() bool {
	return r != nil && (r.IsCheck == "1" || r.State == "3" || r.State == "204")
}

// BrandInfo 保存快递公司名称与客服电话。
type BrandInfo struct {
	Name string `json:"name"` // 快递公司名称。
	Tel  string `json:"tel"`  // 快递公司客服电话。
}

// LastResult 保存上游提供的最近查询结果摘要。
type LastResult struct {
	Message string `json:"message"` // 最近结果的状态描述。
	State   string `json:"state"`   // 物流状态码，例如 3 或 204 表示已签收。
}

// RouteInfo 保存运单起点、当前位置和终点的区域信息。
type RouteInfo struct {
	From RouteLocation `json:"from"` // 发件地区域。
	Cur  RouteLocation `json:"cur"`  // 当前物流区域。
	To   RouteLocation `json:"to"`   // 目的地区域。
}

// RouteLocation 表示物流路由中的一个行政区域节点。
type RouteLocation struct {
	Number string `json:"number"` // 行政区域编码。
	Name   string `json:"name"`   // 行政区域名称。
}
