// 文件用途：定义仅返回操作是否成功的简单响应结构。

package baseresponse

// BoolResponse 表示只需返回操作成功与否的简短业务响应。
type BoolResponse struct {
	// 是否成功应答
	Successful bool `json:"successful"` // 本次操作是否成功。
}
