// 文件用途：从公开 API 验证身份传播、权限读取及商户/用户隔离。

package authentication_test

import (
	"context"
	"testing"

	"github.com/anheicang/xinlin/base/authentication"
)

// TestPublicAuthContextAndDataScopes 从公开 API 验证身份传播、权限读取及商户/用户隔离。
func TestPublicAuthContextAndDataScopes(t *testing.T) {
	ctx := context.Background()
	if authentication.CanAccessShop(ctx, 8) || authentication.CanAccessUser(ctx, 42) {
		t.Fatal("anonymous access allowed")
	}
	ctx = authentication.WithAuthContext(ctx, authentication.AuthContext{
		UID: 42, ClientType: authentication.ClientTypeShop, ShopId: 8, Sid: "example-session",
		Perms: map[string]bool{"example:read": true},
	})
	if authentication.CtxUserId(ctx) != 42 || authentication.CtxShopId(ctx) != 8 || authentication.CtxSessionId(ctx) != "example-session" || authentication.CtxClientType(ctx) != authentication.ClientTypeShop {
		t.Fatal("auth context was not propagated")
	}
	if !authentication.HasPerm(authentication.CtxPerms(ctx), "example:read") {
		t.Fatal("permission missing")
	}
	if !authentication.CanAccessShop(ctx, 8) || authentication.CanAccessShop(ctx, 9) {
		t.Fatal("shop isolation changed")
	}
	if !authentication.CanAccessUser(ctx, 42) || authentication.CanAccessUser(ctx, 43) {
		t.Fatal("user isolation changed")
	}
}
