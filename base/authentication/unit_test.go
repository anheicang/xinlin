package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2/server"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func signedClaims(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestRequestIdentityAndValidation(t *testing.T) {
	for _, value := range []interface{}{json.Number("42"), int64(42), float64(42), "42", nil, true} {
		r := httptest.NewRequest("GET", "/", nil)
		if value != nil {
			r = r.WithContext(context.WithValue(r.Context(), "userId", value))
		}
		r.Header.Set("Authorization", "Bearer token")
		r.Header.Set(AppXUserIdKey, "42")
		token, match, err := GetHttpRequestToken(r)
		valid := value != nil && value != true
		if valid && (err != nil || !match || token != "token") {
			t.Fatalf("identity=%v token=%q match=%v err=%v", value, token, match, err)
		}
		if !valid && err == nil {
			t.Fatal("invalid identity accepted")
		}
		w := httptest.NewRecorder()
		if Validate("token", r, w) != valid {
			t.Fatal("unexpected validation")
		}
		if valid {
			if Validate("different", r, httptest.NewRecorder()) {
				t.Fatal("different token accepted")
			}
			r.Header.Set(AppXUserIdKey, "99")
			if Validate("token", r, httptest.NewRecorder()) {
				t.Fatal("different identity accepted")
			}
		}
	}
}

func TestJWTRejectionPermissionsAndContextFallback(t *testing.T) {
	if token, err := GetJwtToken("secret", 3600, 42); err != nil || token == "" {
		t.Fatal("JWT convenience wrapper failed", err)
	}
	if _, err := ParseToken("malformed", "secret"); err == nil {
		t.Fatal("malformed token accepted")
	}
	expired := signedClaims(t, jwt.MapClaims{"uid": 42, "exp": time.Now().Add(-time.Hour).Unix()})
	if _, err := ParseToken(expired, "secret"); err == nil {
		t.Fatal("expired token accepted")
	}
	for _, tt := range []struct {
		raw  interface{}
		want []string
	}{
		{nil, []string{}}, {[]interface{}{"read", nil, 1, "write"}, []string{"read", "write"}}, {"read", []string{"read"}}, {"", []string{}}, {123, []string{}},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+signedClaims(t, jwt.MapClaims{"uid": 42, "perms": tt.raw}))
		got, err := GetPerms(r, "secret")
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("perms=%v err=%v", got, err)
		}
		ctx := MustUserId(context.Background(), r, "secret")
		if CtxUserId(ctx) != 42 || !reflect.DeepEqual(CtxPerms(ctx), tt.want) {
			t.Fatal("fallback auth context lost")
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	if _, err := GetUserId(r, "secret"); err == nil {
		t.Fatal("missing token accepted")
	}
	if _, err := GetPerms(r, "secret"); err == nil {
		t.Fatal("missing token accepted")
	}
	ctx := context.Background()
	if MustUserId(ctx, r, "secret") != ctx {
		t.Fatal("invalid token changed context")
	}
	ctx = WithAuthContext(ctx, AuthContext{UID: 99})
	if MustUserId(ctx, r, "secret") != ctx {
		t.Fatal("existing context overwritten")
	}
	r.Header.Set("Authorization", signedClaims(t, jwt.MapClaims{}))
	if _, err := GetUserId(r, "secret"); err == nil {
		t.Fatal("missing uid accepted")
	}
}

func TestDataScopeRulesAndEmptyContext(t *testing.T) {
	ctx := context.Background()
	if CtxUserId(ctx) != 0 || len(CtxPerms(ctx)) != 0 || CtxClientType(ctx) != "" || CtxSessionId(ctx) != "" || CtxShopId(ctx) != 0 {
		t.Fatal("invalid empty context defaults")
	}
	if CanAccessShop(ctx, 0) || CanAccessUser(ctx, -1) || HasDataScope(AuthContext{}, DataScopeAll, 0) {
		t.Fatal("invalid target accepted")
	}
	for _, scope := range []DataScope{{Type: DataScopeAll}, {Type: DataScopeShop, Value: 8}, {Type: DataScopeShopList, Value: 8}, {Type: DataScopeSelf, Value: 42}} {
		auth := AuthContext{UID: 1, DataScopes: []DataScope{scope}}
		ctx := WithAuthContext(context.Background(), auth)
		if !HasDataScope(auth, scope.Type, scope.Value) || HasDataScope(auth, 999, 9) || HasDataScope(auth, scope.Type, 999) {
			t.Fatal("scope matching incorrect")
		}
		if scope.Type != DataScopeSelf && !CanAccessShop(ctx, 8) {
			t.Fatal("shop scope rejected")
		}
		if (scope.Type == DataScopeAll || scope.Type == DataScopeSelf) && !CanAccessUser(ctx, 42) {
			t.Fatal("user scope rejected")
		}
	}
	if id := NewSessionId(); id == "" {
		t.Fatal("missing session ID")
	} else if _, err := uuid.Parse(id); err != nil {
		t.Fatal(err)
	}
}

func TestSessionLifecycleAndInvalidArguments(t *testing.T) {
	s, r := testRedis(t)
	ctx := context.Background()
	data := SessionData{UID: 42, ClientType: ClientTypeApp, PermCodes: []string{"read"}}
	if err := SaveSession(ctx, r, "sid", data, 60); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(ctx, r, ClientTypeApp, "sid")
	if err != nil || !reflect.DeepEqual(*got, data) {
		t.Fatalf("session=%+v err=%v", got, err)
	}
	s.Set(BuildSessionKey(ClientTypeApp, "broken"), "not-json")
	if _, err := LoadSession(ctx, r, ClientTypeApp, "broken"); err == nil {
		t.Fatal("malformed session accepted")
	}
	if _, err := LoadSession(ctx, r, ClientTypeApp, "missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("missing session not identified")
	}
	for _, client := range []*redis.Redis{nil, r} {
		if SaveSession(ctx, client, "", data, 60) == nil || SaveSession(ctx, client, "sid", data, 0) == nil || RemoveSession(ctx, client, "", "sid", 42) == nil || RemoveUserSessions(ctx, client, ClientTypeApp, 0) == nil {
			t.Fatal("invalid session arguments accepted")
		}
	}
	if _, err := LoadSession(ctx, nil, ClientTypeApp, "sid"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("invalid client accepted")
	}
	if err := RemoveSession(ctx, r, ClientTypeApp, "sid", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(ctx, r, ClientTypeApp, "sid"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("deleted session load succeeded")
	}
	s.SetError("ERR unavailable")
	if SaveSession(ctx, r, "new", data, 60) == nil {
		t.Fatal("save failure hidden")
	}
}

func TestTokenVersionInputsStorageAndCorruption(t *testing.T) {
	s, r := testRedis(t)
	ctx := context.Background()
	for _, clientType := range []string{ClientTypeAdmin, ClientTypeShop, ClientTypeApp} {
		if v, err := GetTokenVersion(ctx, r, clientType, 8, 42); err != nil || v != 1 {
			t.Fatalf("initial version=%d err=%v", v, err)
		}
	}
	for _, invalid := range []struct {
		kind      string
		shop, uid int64
	}{{"bad", 0, 1}, {ClientTypeShop, -1, 1}, {ClientTypeApp, 0, 0}} {
		if _, err := GetTokenVersion(ctx, r, invalid.kind, invalid.shop, invalid.uid); err == nil {
			t.Fatal("invalid version input accepted")
		}
		if _, err := IncrTokenVersion(ctx, r, invalid.kind, invalid.shop, invalid.uid); err == nil {
			t.Fatal("invalid increment accepted")
		}
	}
	if _, err := GetTokenVersion(ctx, nil, ClientTypeApp, 0, 1); err == nil {
		t.Fatal("nil Redis accepted")
	}
	if _, err := IncrTokenVersion(ctx, nil, ClientTypeApp, 0, 1); err == nil {
		t.Fatal("nil Redis accepted")
	}
	key, _ := buildTokenVersionKey(ClientTypeAdmin, 0, 42)
	for _, corrupt := range []string{"invalid", "0", "-1"} {
		s.Set(key, corrupt)
		if _, err := GetTokenVersion(ctx, r, ClientTypeAdmin, 0, 42); err == nil {
			t.Fatal("corrupt version accepted")
		}
	}
	s.SetError("ERR unavailable")
	if _, err := GetTokenVersion(ctx, r, ClientTypeApp, 0, 1); err == nil {
		t.Fatal("read failure hidden")
	}
	if _, err := IncrTokenVersion(ctx, r, ClientTypeApp, 0, 1); err == nil {
		t.Fatal("increment failure hidden")
	}
	s.SetError("")
	s.Server().SetPreHook(func(p *server.Peer, cmd string, args ...string) bool {
		if cmd == "SETNX" || cmd == "SET" {
			p.WriteError("ERR writes blocked")
			return true
		}
		return false
	})
	if _, err := GetTokenVersion(ctx, r, ClientTypeAdmin, 0, 999); err == nil {
		t.Fatal("initialization failure hidden")
	}
}
