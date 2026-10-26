package authentication

import "context"

const (
	ClientTypeAdmin = "admin"
	ClientTypeShop  = "shop"
	ClientTypeApp   = "app"
)

const (
	DataScopeAll      = 1
	DataScopeShop     = 2
	DataScopeShopList = 3
	DataScopeSelf     = 5
)

type ctxKey string

const (
	ctxKeyAuthContext ctxKey = "authContext"
	ctxKeyUserId      ctxKey = "userId"
	ctxKeyPerms       ctxKey = "perms"
)

type AuthContext struct {
	UID          int64
	ClientType   string // admin|shop|app
	RoleIds      []int64
	Perms        map[string]bool
	ShopId       int64
	DataScopes   []DataScope
	Sid          string
	IsSuperAdmin bool // 是否为系统超级管理员
	IsShopAdmin  bool // 是否为商户管理员
}

type DataScope struct {
	Type  int
	Value int64
}

func WithAuthContext(ctx context.Context, authCtx AuthContext) context.Context {
	ctx = context.WithValue(ctx, ctxKeyAuthContext, authCtx)
	ctx = context.WithValue(ctx, ctxKeyUserId, authCtx.UID)
	ctx = context.WithValue(ctx, ctxKeyPerms, PermsToSlice(authCtx.Perms))
	return ctx
}

func CtxAuth(ctx context.Context) (AuthContext, bool) {
	val := ctx.Value(ctxKeyAuthContext)
	authCtx, ok := val.(AuthContext)
	return authCtx, ok
}

func CtxUserId(ctx context.Context) int64 {
	userId, ok := ctx.Value(ctxKeyUserId).(int64)
	if !ok {
		return 0
	}
	return userId
}

func CtxPerms(ctx context.Context) []string {
	perms, ok := ctx.Value(ctxKeyPerms).([]string)
	if !ok {
		return []string{}
	}
	return perms
}

func CtxClientType(ctx context.Context) string {
	authCtx, ok := CtxAuth(ctx)
	if !ok {
		return ""
	}
	return authCtx.ClientType
}

func CtxSessionId(ctx context.Context) string {
	authCtx, ok := CtxAuth(ctx)
	if !ok {
		return ""
	}
	return authCtx.Sid
}

func CtxShopId(ctx context.Context) int64 {
	authCtx, ok := CtxAuth(ctx)
	if !ok {
		return 0
	}
	return authCtx.ShopId
}

func PermsToSlice(perms map[string]bool) []string {
	if len(perms) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(perms))
	for p := range perms {
		out = append(out, p)
	}
	return out
}
