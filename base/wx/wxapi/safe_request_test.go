package wxapi

import (
	"context"
	"errors"
	"fmt"
	"github.com/anheicang/xinlin/base/baseerror"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"
)

type fakeTransport func(*http.Request) (*http.Response, error)

func (f fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWXTransportErrorsNeverExposeCredentialURLOrNestedError(t *testing.T) {
	for _, cause := range []error{errors.New("upstream PRIVATE_NESTED"), context.DeadlineExceeded, context.Canceled} {
		client := resty.New().SetTransport(fakeTransport(func(r *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: r.URL.String(), Err: cause}
		}))
		_, err := wxRequest(client.R(), http.MethodPost, "https://api.weixin.qq.com/wxa/msg_sec_check?access_token=PRIVATE_TOKEN&secret=PRIVATE_SECRET&code=PRIVATE_CODE")
		if err == nil {
			t.Fatal("transport failure disappeared")
		}
		for _, formatted := range []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
			if strings.Contains(formatted, "PRIVATE_") || strings.Contains(formatted, "access_token=") {
				t.Fatal("credential URL escaped through error")
			}
		}
		if errors.Unwrap(err) != nil {
			t.Fatal("unsafe original error can still be unwrapped")
		}
		if !strings.Contains(err.Error(), "/wxa/msg_sec_check") {
			t.Fatal("safe endpoint missing")
		}
	}
}

func TestWXRequestPreservesSuccessAndBusinessErrorCode(t *testing.T) {
	client := resty.New().SetTransport(fakeTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("access_token") != "PRIVATE_TOKEN" {
			t.Error("request credential removed")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"PRIVATE_RESPONSE"}`)), Request: r}, nil
	}))
	response, err := wxRequest(client.R(), http.MethodGet, "https://api.weixin.qq.com/cgi-bin/token?access_token=PRIVATE_TOKEN")
	if err != nil || !strings.Contains(string(response.Body()), "PRIVATE_RESPONSE") {
		t.Fatal("successful response changed")
	}
	codeErr, ok := baseerror.FromError(wxResponseError(40001))
	if !ok || codeErr.Code() != 40001 {
		t.Fatal("WeChat business error code changed")
	}
}
