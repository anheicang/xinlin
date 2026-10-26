package wxapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/anheicang/xinlin/base/baseerror"
	"github.com/go-resty/resty/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func mockWX(t *testing.T, handler func(*http.Request) string) {
	t.Helper()
	previous := newWXClient
	newWXClient = func() *resty.Client {
		return resty.New().SetTransport(fakeTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(handler(r)))}, nil
		}))
	}
	t.Cleanup(func() { newWXClient = previous })
}

func TestOAuthUsesEachAuthorizationCodeDespiteOldCache(t *testing.T) {
	s := miniredis.RunT(t)
	s.Set("open_platform_web_access_token", "old-user-token")
	r, err := redis.NewRedis(redis.RedisConf{Host: s.Addr(), Type: "node"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	mockWX(t, func(r *http.Request) string {
		calls++
		q := r.URL.Query()
		if q.Get("appid") != "app" || q.Get("secret") != "secret&value" || q.Get("grant_type") != "authorization_code" {
			t.Error("OAuth parameters corrupted")
		}
		body, _ := json.Marshal(map[string]string{"openid": "user-" + q.Get("code")})
		return string(body)
	})
	h := &OpenPlatformWebHelper{RedisConn: r, C: Config{AppId: "app", Secret: "secret&value"}}
	for _, code := range []string{"first", "second+&code"} {
		id, err := h.GetOpenId(code)
		if err != nil || id != "user-"+code {
			t.Fatalf("openid=%q err=%v", id, err)
		}
	}
	if calls != 2 {
		t.Fatalf("expected two independent OAuth exchanges, got %d", calls)
	}
	if len(s.Keys()) != 1 {
		t.Fatal("OAuth credentials unexpectedly cached")
	}
}

func TestOAuthRejectsBusinessErrorsAndMissingIdentity(t *testing.T) {
	for _, body := range []string{`{"errcode":40001,"errmsg":"PRIVATE"}`, `{}`, `broken-json`} {
		t.Run(body, func(t *testing.T) {
			mockWX(t, func(*http.Request) string { return body })
			h := &OpenPlatformWebHelper{C: Config{AppId: "app", Secret: "secret"}}
			if _, err := h.GetOpenId("code"); err == nil || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("invalid error: %v", err)
			}
		})
	}
}

func TestOfficialAccountChecksBusinessErrorCodes(t *testing.T) {
	for _, body := range []string{`{"errcode":40001,"errmsg":"PRIVATE"}`, `{"openid":"user","count":1,"data":{"openid":["user"]}}`} {
		t.Run(body, func(t *testing.T) {
			mockWX(t, func(*http.Request) string { return body })
			_, userErr := GetUserInfo("user", "token")
			_, listErr := GetFollowUserList("token")
			for _, err := range []error{userErr, listErr} {
				if strings.Contains(body, "errcode") {
					codeErr, ok := baseerror.FromError(err)
					if !ok || codeErr.Code() != 40001 || strings.Contains(err.Error(), "PRIVATE") {
						t.Fatalf("business error lost: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
