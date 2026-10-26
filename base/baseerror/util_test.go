// 文件用途：验证业务错误字段、HTTP 状态、自定义错误和顺序操作遇错短路。

package baseerror

import (
	"errors"
	"testing"
)

// customBusiness 模拟外部自定义业务错误，验证接口式错误识别。
type customBusiness struct{}

// Error 返回自定义业务错误样本的描述。
func (customBusiness) Error() string { return "custom" }

// Code 返回自定义业务错误样本的业务码。
func (customBusiness) Code() int64 { return 88 }

// Desc 返回自定义业务错误样本的对外描述。
func (customBusiness) Desc() string { return "custom" }

// Data 返回自定义业务错误样本的附加数据。
func (customBusiness) Data() interface{} { return "extra" }

// TestErrorFieldsAndGleanShortCircuit 验证业务错误字段、HTTP 状态、自定义错误和顺序操作遇错短路。
func TestErrorFieldsAndGleanShortCircuit(t *testing.T) {
	if NewHttpCodeError(403, 42, "denied").HttpStatus() != 403 {
		t.Fatal("HTTP status lost")
	}
	err := NewError(42, "description", "data")
	if err.Error() != "description" || err.Desc() != "description" || err.Data() != "data" || err.Code() != 42 {
		t.Fatal("error fields lost")
	}
	if NewDefaultError("default").Code() != 10001 || ErrDefault != ErrSystem {
		t.Fatal("default error contract changed")
	}
	calls := 0
	want := errors.New("stop")
	got := Glean(func() error { calls++; return nil }, func() error { calls++; return want }, func() error { calls++; return nil })
	if !errors.Is(got, want) || calls != 2 {
		t.Fatal("Glean did not stop on first error")
	}
	if Glean() != nil || Glean(func() error { return nil }) != nil {
		t.Fatal("successful operations failed")
	}
	if got, ok := FromError(customBusiness{}); !ok || got.Code() != 88 || got.Data() != "extra" {
		t.Fatal("custom business error lost")
	}
	var nilCode *CodeError
	var nilHTTP *HttpCodeError
	for _, err := range []error{nil, nilCode, nilHTTP} {
		if _, ok := FromError(err); ok {
			t.Fatal("nil business error accepted")
		}
	}
}
