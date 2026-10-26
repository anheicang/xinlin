// 文件用途：回归验证值为 false 的权限不会进入有效权限列表；回归验证 JWT 用户 ID 精度、签发时间、有效期和签名拒绝。

package authentication

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/golang-jwt/jwt/v4"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// testRedis 启动独立内存 Redis 并注册测试清理，隔离会话/版本测试数据。
func testRedis(t *testing.T) (*miniredis.Miniredis, *redis.Redis) {
	t.Helper()
	s := miniredis.RunT(t)
	r, err := redis.NewRedis(redis.RedisConf{Host: s.Addr(), Type: "node"})
	if err != nil {
		t.Fatal(err)
	}
	return s, r
}

// TestDisabledPermissionsAreExcluded 回归验证值为 false 的权限不会进入有效权限列表。
func TestDisabledPermissionsAreExcluded(t *testing.T) {
	ctx := WithAuthContext(context.Background(), AuthContext{UID: 1, Perms: map[string]bool{"read": true, "delete": false}})
	if !HasPerm(CtxPerms(ctx), "read") || HasPerm(CtxPerms(ctx), "delete") {
		t.Fatal("incorrect permission conversion")
	}
}

// TestJWTIdentityAndIssuedAt 回归验证 JWT 用户 ID 精度、签发时间、有效期和签名拒绝。
func TestJWTIdentityAndIssuedAt(t *testing.T) {
	const secret = "test-secret"
	for _, id := range []int64{1, 9007199254740993, 9223372036854775807} {
		for _, legacy := range []bool{false, true} {
			before := time.Now().Unix()
			var token string
			var err error
			if legacy {
				token, err = GetJwtTokenWithPerms(secret, 3600, id, []string{"read"})
			} else {
				token, err = GetJwtTokenWithSession(secret, 3600, id, ClientTypeApp, "sid")
			}
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			got, err := GetUserId(r, secret)
			if err != nil || got != id {
				t.Fatalf("uid=%d got=%d err=%v", id, got, err)
			}
			claims, err := ParseToken(token, secret)
			if err != nil {
				t.Fatal(err)
			}
			iat, err := claims["iat"].(json.Number).Int64()
			if err != nil || iat < before || iat > time.Now().Unix() {
				t.Fatalf("invalid iat=%v", claims["iat"])
			}
			exp, _ := claims["exp"].(json.Number).Int64()
			if exp-iat != 3600 {
				t.Fatal("wrong expiration")
			}
			if _, err := ParseToken(token, "wrong-secret"); err == nil {
				t.Fatal("invalid signature accepted")
			}
		}
	}
	for _, uid := range []interface{}{1.5, "invalid", json.Number("9223372036854775808")} {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"uid": uid, "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		if _, err := GetUserId(r, secret); err == nil {
			t.Fatalf("invalid uid accepted: %v", uid)
		}
	}
}

// TestSessionIndexCoversLongestSession 回归验证短会话不会缩短长会话索引，批量撤销可以找到长会话。
func TestSessionIndexCoversLongestSession(t *testing.T) {
	for _, lifetimes := range [][]int64{{3600, 10}, {10, 3600}} {
		s, r := testRedis(t)
		ctx := context.Background()
		for i, ttl := range lifetimes {
			if err := SaveSession(ctx, r, fmt.Sprint(i), SessionData{UID: 1, ClientType: ClientTypeApp}, ttl); err != nil {
				t.Fatal(err)
			}
		}
		s.FastForward(11 * time.Second)
		if err := RemoveUserSessions(ctx, r, ClientTypeApp, 1); err != nil {
			t.Fatal(err)
		}
		for i := range lifetimes {
			if s.Exists(BuildSessionKey(ClientTypeApp, fmt.Sprint(i))) {
				t.Fatal("session survives user logout")
			}
		}
	}
}

// TestSessionOperationsPropagateRedisErrors 回归验证会话读写/撤销不会吞掉 Redis 故障。
func TestSessionOperationsPropagateRedisErrors(t *testing.T) {
	s, r := testRedis(t)
	ctx := context.Background()
	if err := SaveSession(ctx, r, "sid", SessionData{UID: 1, ClientType: ClientTypeApp}, 3600); err != nil {
		t.Fatal(err)
	}
	s.SetError("ERR forced failure")
	if err := RemoveSession(ctx, r, ClientTypeApp, "sid", 1); err == nil {
		t.Fatal("logout failure hidden")
	}
	if err := RemoveUserSessions(ctx, r, ClientTypeApp, 1); err == nil {
		t.Fatal("user logout failure hidden")
	}
	if _, err := LoadSession(ctx, r, ClientTypeApp, "sid"); err == nil || err == ErrSessionNotFound {
		t.Fatal("storage failure hidden")
	}
	s.SetError("")
	if err := RemoveSession(ctx, r, ClientTypeApp, "sid", 1); err != nil {
		t.Fatal(err)
	}
	if s.Exists(BuildSessionKey(ClientTypeApp, "sid")) {
		t.Fatal("session not deleted")
	}
}

// TestRevocationStartsAboveDefaultAndIsAtomic 回归验证首次撤销版本高于 1，且并发递增不重复。
func TestRevocationStartsAboveDefaultAndIsAtomic(t *testing.T) {
	_, r := testRedis(t)
	const workers = 12
	var wg sync.WaitGroup
	versions := make(chan int64, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := IncrTokenVersion(context.Background(), r, ClientTypeAdmin, 0, 1)
			if err != nil {
				t.Error(err)
				return
			}
			versions <- v
		}()
	}
	wg.Wait()
	close(versions)
	seen := map[int64]bool{}
	for v := range versions {
		if v <= 1 || seen[v] {
			t.Fatalf("invalid/repeated version %d", v)
		}
		seen[v] = true
	}
	if len(seen) != workers {
		t.Fatal("missing version increments")
	}
	v, err := GetTokenVersion(context.Background(), r, ClientTypeAdmin, 0, 1)
	if err != nil || v != workers+1 {
		t.Fatalf("version=%d err=%v", v, err)
	}
}

// TestSessionDeletionFailuresKeepRetryIndex 验证删除或索引更新失败时保留可重试索引，恢复后可完成撤销。
func TestSessionDeletionFailuresKeepRetryIndex(t *testing.T) {
	for _, command := range []string{"DEL", "SREM"} {
		t.Run(command, func(t *testing.T) {
			s, r := testRedis(t)
			ctx := context.Background()
			if err := SaveSession(ctx, r, "sid", SessionData{UID: 1, ClientType: ClientTypeApp}, 3600); err != nil {
				t.Fatal(err)
			}
			s.Server().SetPreHook(func(peer *server.Peer, cmd string, args ...string) bool {
				if cmd == command {
					peer.WriteError("ERR deletion blocked")
					return true
				}
				return false
			})
			if err := RemoveUserSessions(ctx, r, ClientTypeApp, 1); err == nil {
				t.Fatal("deletion failure hidden")
			}
			if !s.Exists(BuildSessionIndexKey(ClientTypeApp, 1)) {
				t.Fatal("retry index lost")
			}
			s.Server().SetPreHook(nil)
			if err := RemoveUserSessions(ctx, r, ClientTypeApp, 1); err != nil {
				t.Fatal(err)
			}
			if s.Exists(BuildSessionKey(ClientTypeApp, "sid")) {
				t.Fatal("session survives retry")
			}
		})
	}
}

// TestSessionIndexFailureRollsBackNewSession 验证索引写入失败时回滚新会话，避免遗留无法批量撤销的登录。
func TestSessionIndexFailureRollsBackNewSession(t *testing.T) {
	s, r := testRedis(t)
	s.Server().SetPreHook(func(peer *server.Peer, cmd string, args ...string) bool {
		if cmd == "EVAL" {
			peer.WriteError("ERR indexing blocked")
			return true
		}
		return false
	})
	err := SaveSession(context.Background(), r, "sid", SessionData{UID: 1, ClientType: ClientTypeApp}, 3600)
	if err == nil || s.Exists(BuildSessionKey(ClientTypeApp, "sid")) {
		t.Fatal("unindexed new session not rolled back")
	}
}
