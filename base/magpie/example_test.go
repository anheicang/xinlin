// 文件用途：展示业务层如何传入提示词并取得结构化结果，示例使用本地模拟网关可直接运行。
package magpie_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/anheicang/xinlin/base/magpie"
)

// ExampleClient_ChatJSON 展示独立业务结果结构；不把业务规则硬编码进公共库。
func ExampleClient_ChatJSON() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"本次数据存在两项差异\"}"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	client, err := magpie.New(magpie.Config{BaseURL: server.URL + "/v1", Model: "group/model"})
	if err != nil {
		panic(err)
	}
	var result struct {
		Summary string `json:"summary"`
	}
	_, err = client.ChatJSON(context.Background(), magpie.ChatRequest{
		Messages: []magpie.Message{
			{Role: "system", Content: "仅根据用户数据输出 JSON，包含 summary 字段。"},
			{Role: "user", Content: "请比较这两组数据……"},
		},
	}, &result)
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Summary)
	// Output: 本次数据存在两项差异
}
