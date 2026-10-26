// 文件用途：定义认证上下文、客户端与数据范围常量，并提供类型安全的身份读取入口。

package authentication

import "context"

const (
	// ClientTypeAdmin 表示平台管理端身份。
	ClientTypeAdmin = "admin"
	// ClientTypeShop 表示商户管理端身份。
	ClientTypeShop = "shop"
	// ClientTypeApp 表示小程序或应用会员身份。
	ClientTypeApp = "app"
)

const (
	// DataScopeAll 表示可访问全部业务数据的授权范围。
	DataScopeAll = 1
	// DataScopeShop 表示仅可访问指定商户的数据。
	DataScopeShop = 2
	// DataScopeShopList 表示商户集合中的一项；多个 DataScope 条目组成集合。
	DataScopeShopList = 3
	// DataScopeSelf 表示与某个用户相关的个人数据范围。
	DataScopeSelf = 5
)

// ctxKey 作为私有 context key 类型，防止不同包使用字符串 key 时发生冲突。
type ctxKey string

const (
	ctxKeyAuthContext ctxKey = "authContext"
	ctxKeyUserId      ctxKey = "userId"
	ctxKeyPerms       ctxKey = "perms"
)

// AuthContext 保存经过业务中间件校验的当前身份、权限和数据范围。
// 管理员标记本身不会自动绕过本包的数据范围检查。
type AuthContext struct {
	UID          int64           // 当前用户 ID，使用 int64 保存。
	ClientType   string          // admin|shop|app
	RoleIds      []int64         // 角色 ID 列表，具体角色含义由业务项目定义。
	Perms        map[string]bool // 权限码到是否允许的映射，只有 true 表示授权。
	ShopId       int64           // 所属商户 ID；没有商户身份时为 0。
	DataScopes   []DataScope     // 服务端授予的数据访问范围。
	Sid          string          // 当前登录会话 ID，与 JWT 的 sid 对应。
	IsSuperAdmin bool            // 是否为系统超级管理员
	IsShopAdmin  bool            // 是否为商户管理员
}

// DataScope 描述一种数据访问范围，Type 表示范围种类，Value 为商户或用户等目标 ID。
type DataScope struct {
	Type  int   // 数据范围类型，对应 DataScope 常量。
	Value int64 // 范围目标 ID，例如商户 ID 或用户 ID。
}

// WithAuthContext 将可信认证结果及常用身份字段写入新 context。
// 应由完成 JWT/会话校验的中间件调用，Perms 中只有值为 true 的权限会进入权限列表。
func WithAuthContext(ctx context.Context, authCtx AuthContext) context.Context {
	ctx = context.WithValue(ctx, ctxKeyAuthContext, authCtx)
	ctx = context.WithValue(ctx, ctxKeyUserId, authCtx.UID)
	ctx = context.WithValue(ctx, ctxKeyPerms, PermsToSlice(authCtx.Perms))
	return ctx
}

// CtxAuth 读取完整认证上下文；尚未注入或值类型不符时返回 false。
func CtxAuth(ctx context.Context) (AuthContext, bool) {
	val := ctx.Value(ctxKeyAuthContext)
	authCtx, ok := val.(AuthContext)
	return authCtx, ok
}

// CtxUserId 读取已注入的用户 ID；没有身份时返回 0。
func CtxUserId(ctx context.Context) int64 {
	userId, ok := ctx.Value(ctxKeyUserId).(int64)
	if !ok {
		return 0
	}
	return userId
}

// CtxPerms 读取已注入的有效权限码；没有权限信息时返回空切片。
func CtxPerms(ctx context.Context) []string {
	perms, ok := ctx.Value(ctxKeyPerms).([]string)
	if !ok {
		return []string{}
	}
	return perms
}

// CtxClientType 读取 admin、shop 或 app 客户端标识；未认证时返回空字符串。
func CtxClientType(ctx context.Context) string {
	authCtx, ok := CtxAuth(ctx)
	if !ok {
		return ""
	}
	return authCtx.ClientType
}

// CtxSessionId 读取当前登录会话 ID，供业务中间件查验 Redis 登录状态。
func CtxSessionId(ctx context.Context) string {
	authCtx, ok := CtxAuth(ctx)
	if !ok {
		return ""
	}
	return authCtx.Sid
}

// CtxShopId 读取当前身份所属商户 ID；无商户身份时返回 0。
func CtxShopId(ctx context.Context) int64 {
	authCtx, ok := CtxAuth(ctx)
	if !ok {
		return 0
	}
	return authCtx.ShopId
}

// PermsToSlice 将权限映射中值为 true 的项转为权限码列表。
// 返回顺序不固定，值为 false 的项不会被授予权限。
func PermsToSlice(perms map[string]bool) []string {
	if len(perms) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(perms))
	for p, allowed := range perms {
		if allowed {
			out = append(out, p)
		}
	}
	return out
}
