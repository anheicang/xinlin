package baseerror

import (
	"errors"
	"fmt"
	"testing"
)

func TestBusinessErrorsSurviveWrapping(t *testing.T) {
	for _, original := range []error{NewCodeError(42, "business"), NewHttpCodeError(403, 42, "business")} {
		wrapped := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", original))
		if !IsCodeError(wrapped) {
			t.Fatal("wrapped business error not recognized")
		}
		got, ok := FromError(wrapped)
		if !ok || got.Code() != 42 || ToCodeError(wrapped).Code() != 42 {
			t.Fatal("business code lost")
		}
	}
	if IsCodeError(errors.New("unknown")) || ToCodeError(nil) != ErrDefault {
		t.Fatal("unknown error accepted")
	}
}
