package baseerror

import (
	"errors"
	"testing"
)

type customBusiness struct{}

func (customBusiness) Error() string     { return "custom" }
func (customBusiness) Code() int64       { return 88 }
func (customBusiness) Desc() string      { return "custom" }
func (customBusiness) Data() interface{} { return "extra" }

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
