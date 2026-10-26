package wxapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/go-resty/resty/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func wxRedis(t *testing.T) (*miniredis.Miniredis, *WxHelper, *OpenPlatformWebHelper, Config) {
	t.Helper()
	s := miniredis.RunT(t)
	c := Config{AppId: "app", Secret: "secret", RedisConfig: redis.RedisConf{Host: s.Addr(), Type: "node"}}
	w := NewWxHelper(c.RedisConfig)
	o, err := NewOpenPlatformWebHelper(c)
	if w == nil || err != nil {
		t.Fatal("Redis client initialization failed", err)
	}
	return s, w, o, c
}

func mockWXResponse(t *testing.T, status int, body string, failure error) {
	t.Helper()
	previous := newWXClient
	newWXClient = func() *resty.Client {
		return resty.New().SetTransport(fakeTransport(func(*http.Request) (*http.Response, error) {
			if failure != nil {
				return nil, failure
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}))
	}
	t.Cleanup(func() { newWXClient = previous })
}

func TestWXConstructorsAndClientDefaults(t *testing.T) {
	if newWXClient().GetClient().Timeout <= 0 {
		t.Fatal("request timeout missing")
	}
	if NewWxHelper(redis.RedisConf{}) != nil {
		t.Fatal("invalid Redis configuration accepted")
	}
	if _, err := NewOpenPlatformWebHelper(Config{}); err == nil {
		t.Fatal("invalid Redis configuration accepted")
	}
	var nilHelper *OpenPlatformWebHelper
	if _, err := nilHelper.GetOpenId("code"); err == nil {
		t.Fatal("nil helper accepted")
	}
}

func TestWXMethodsSuccessAndPayloads(t *testing.T) {
	_, w, o, c := wxRedis(t)
	mockWX(t, func(req *http.Request) string {
		switch req.URL.Path {
		case "/cgi-bin/token":
			return `{"access_token":"token"}`
		case "/sns/jscode2session":
			if req.URL.Query().Get("js_code") != "login-code" {
				t.Error("login code lost")
			}
			return `{"openid":"user"}`
		case "/wxa/business/getuserphonenumber":
			body, _ := io.ReadAll(req.Body)
			if string(body) != `{"code":"phone-code"}` {
				t.Errorf("phone payload=%s", body)
			}
			return `{"phone_info":{"phoneNumber":"fixture-phone"}}`
		case "/cgi-bin/message/subscribe/send":
			body, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(body), `"template_id":"template"`) || !strings.Contains(string(body), `"touser":"user"`) {
				t.Error("subscription payload lost")
			}
			return `{"errcode":0}`
		case "/wxa/msg_sec_check":
			return `{"errcode":0,"result":{"suggest":"pass"}}`
		case "/wxa/media_check_async":
			return `{"errcode":0}`
		}
		t.Errorf("unexpected endpoint %s", req.URL.Path)
		return `{}`
	})
	if result, err := w.Login("login-code", c); err != nil || result.OpenId != "user" {
		t.Fatalf("login=%+v err=%v", result, err)
	}
	if result, err := w.GetPhoneData("phone-code", c); err != nil || result.PhoneInfo.PhoneNumber != "fixture-phone" {
		t.Fatalf("phone=%+v err=%v", result, err)
	}
	if _, err := o.SendSubscribe(c, Subscribe{TemplateId: "template", Touser: "user"}); err != nil {
		t.Fatal(err)
	}
	if result, err := o.CheckTextLegal(c, "text", "user", 1); err != nil || result.Result.Suggest != "pass" {
		t.Fatalf("check=%+v err=%v", result, err)
	}
	if _, err := o.CheckMediaLegal(c, "https://fixture.invalid/image", "user", 1, 2); err != nil {
		t.Fatal(err)
	}
}

func TestWXFailureMatrix(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{"http", 503, `{}`, nil}, {"decode", 200, `not-json`, nil}, {"business", 200, `{"errcode":40003}`, nil}, {"transport", 0, "", errors.New("transport unavailable")},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s, w, o, c := wxRedis(t)
			s.Set(fmt.Sprintf(AccessTokenKey, c.AppId), "cached")
			mockWXResponse(t, scenario.status, scenario.body, scenario.err)
			for _, operation := range []func() error{
				func() error { _, err := w.Login("code", c); return err },
				func() error { _, err := w.GetPhoneData("code", c); return err },
				func() error { _, err := o.SendSubscribe(c, Subscribe{}); return err },
				func() error { _, err := o.CheckTextLegal(c, "text", "user", 1); return err },
				func() error { _, err := o.CheckMediaLegal(c, "url", "user", 1, 2); return err },
				func() error { _, err := o.GetOpenId("code"); return err },
				func() error { _, err := GetAccessToken(c.AppId, c.Secret); return err },
				func() error { _, err := GetUserInfo("user", "token"); return err },
				func() error { _, err := GetFollowUserList("token"); return err },
			} {
				if err := operation(); err == nil {
					t.Fatal("upstream failure hidden")
				}
			}
		})
	}
}

func TestWXTokenMissAndCacheWriteFailures(t *testing.T) {
	for _, failCache := range []bool{false, true} {
		t.Run(fmt.Sprint(failCache), func(t *testing.T) {
			s, w, o, c := wxRedis(t)
			if failCache {
				s.Server().SetPreHook(func(p *server.Peer, cmd string, args ...string) bool {
					if cmd == "SETEX" || cmd == "SET" {
						p.WriteError("ERR cache writes blocked")
						return true
					}
					return false
				})
			}
			mockWX(t, func(r *http.Request) string {
				if r.URL.Path == "/cgi-bin/token" {
					return `{"access_token":"token"}`
				}
				return `{"errcode":0}`
			})
			for _, operation := range []func() error{
				func() error { _, e := w.GetPhoneData("code", c); return e },
				func() error { _, e := o.SendSubscribe(c, Subscribe{}); return e },
				func() error { _, e := o.CheckTextLegal(c, "text", "user", 1); return e },
				func() error { _, e := o.CheckMediaLegal(c, "url", "user", 1, 2); return e },
			} {
				s.Del(fmt.Sprintf(AccessTokenKey, c.AppId))
				if err := operation(); (err != nil) != failCache {
					t.Fatalf("cacheFailure=%v err=%v", failCache, err)
				}
			}
		})
	}
	_, w, o, c := wxRedis(t)
	mockWXResponse(t, 200, `{"errcode":40003}`, nil)
	if _, err := w.GetPhoneData("code", c); err == nil {
		t.Fatal("token fetch error hidden")
	}
	if _, err := o.SendSubscribe(c, Subscribe{}); err == nil {
		t.Fatal("token fetch error hidden")
	}
	if _, err := o.CheckTextLegal(c, "", "", 1); err == nil {
		t.Fatal("token fetch error hidden")
	}
	if _, err := o.CheckMediaLegal(c, "", "", 1, 1); err == nil {
		t.Fatal("token fetch error hidden")
	}
}

func TestPhoneTokenRefreshAndRetryFailures(t *testing.T) {
	for _, secondBody := range []string{`{"phone_info":{"phoneNumber":"phone"}}`, `{"errcode":40003}`, `not-json`} {
		t.Run(secondBody, func(t *testing.T) {
			s, w, _, c := wxRedis(t)
			s.Set(fmt.Sprintf(AccessTokenKey, c.AppId), "old")
			calls := 0
			mockWX(t, func(r *http.Request) string {
				if r.URL.Path == "/cgi-bin/token" {
					return `{"access_token":"new"}`
				}
				calls++
				if calls == 1 {
					return `{"errcode":42001}`
				}
				return secondBody
			})
			result, err := w.GetPhoneData("code", c)
			if strings.Contains(secondBody, "phone_info") {
				if err != nil || result.PhoneInfo.PhoneNumber != "phone" {
					t.Fatal("refresh failed", err)
				}
			} else if err == nil {
				t.Fatal("retry failure hidden")
			}
			if calls != 2 {
				t.Fatalf("retry count=%d", calls)
			}
		})
	}
	_, w, _, c := wxRedis(t)
	mockWX(t, func(r *http.Request) string {
		if r.URL.Path == "/cgi-bin/token" {
			return `{"errcode":40003}`
		}
		return `{"errcode":40001}`
	})
	w.RedisConn.Setex(fmt.Sprintf(AccessTokenKey, c.AppId), "old", 60)
	if _, err := w.GetPhoneData("code", c); err == nil {
		t.Fatal("refresh error hidden")
	}
}
