package kd100

// QueryRequest 是快递100实时查询请求。
type QueryRequest struct {
	Company  string `json:"com"`
	Number   string `json:"num"`
	Phone    string `json:"phone,omitempty"`
	To       string `json:"to,omitempty"`
	ResultV2 string `json:"resultv2"`
}

func NewQueryRequest(company, number, phone string) *QueryRequest {
	return &QueryRequest{Company: company, Number: number, Phone: phone, ResultV2: "4"}
}

type TrackData struct {
	Time       string `json:"time"`
	Ftime      string `json:"ftime"`
	Context    string `json:"context"`
	Location   string `json:"location"`
	Status     string `json:"status"`
	AreaCode   string `json:"areaCode"`
	AreaName   string `json:"areaName"`
	AreaCenter string `json:"areaCenter"`
	AreaPinYin string `json:"areaPinYin"`
}

type QueryResponse struct {
	Message    string      `json:"message"`
	State      string      `json:"state"`
	Status     string      `json:"status"`
	ReturnCode string      `json:"returnCode"`
	Condition  string      `json:"condition"`
	IsCheck    string      `json:"ischeck"`
	Com        string      `json:"com"`
	Nu         string      `json:"nu"`
	Data       []TrackData `json:"data"`
	RouteInfo  RouteInfo   `json:"routeInfo"`
	ReturnFlag string      `json:"returnFlag"`
	ComOld     string      `json:"comOld"`
	BrandInfo  BrandInfo   `json:"brandInfo"`
	LastResult LastResult  `json:"lastResult"`
}

func (r *QueryResponse) APIStatus() string {
	if r == nil {
		return ""
	}
	if r.ReturnCode != "" {
		return r.ReturnCode
	}
	return r.Status
}

func (r *QueryResponse) IsSuccess() bool {
	return r != nil && r.Message == "ok" && r.APIStatus() == "200"
}

func (r *QueryResponse) IsSigned() bool {
	return r != nil && (r.IsCheck == "1" || r.State == "3" || r.State == "204")
}

type BrandInfo struct {
	Name string `json:"name"`
	Tel  string `json:"tel"`
}

type LastResult struct {
	Message string `json:"message"`
	State   string `json:"state"`
}

type RouteInfo struct {
	From RouteLocation `json:"from"`
	Cur  RouteLocation `json:"cur"`
	To   RouteLocation `json:"to"`
}

type RouteLocation struct {
	Number string `json:"number"`
	Name   string `json:"name"`
}
