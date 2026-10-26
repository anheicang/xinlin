// 文件用途：根据已校验的用户身份和授权范围判断商户、用户数据能否访问。

package authentication

import "context"

// HasDataScope 判断身份是否包含指定数据范围。
// UID 为 0 时拒绝；value 为 0 表示只匹配范围类型，否则还需匹配范围值。
func HasDataScope(authCtx AuthContext, scopeType int, value int64) bool {
	if authCtx.UID == 0 {
		return false
	}
	for _, scope := range authCtx.DataScopes {
		if scope.Type != scopeType {
			continue
		}
		if value == 0 || scope.Value == value {
			return true
		}
	}
	return false
}

// CanAccessShop 判断上下文身份能否访问指定商户。
// 允许本商户、全部数据范围及显式商户范围；无身份或 shopId 非正数时拒绝。
func CanAccessShop(ctx context.Context, shopId int64) bool {
	if shopId <= 0 {
		return false
	}
	authCtx, ok := CtxAuth(ctx)
	if !ok {
		return false
	}
	if authCtx.ShopId > 0 && authCtx.ShopId == shopId {
		return true
	}
	if HasDataScope(authCtx, DataScopeAll, 0) {
		return true
	}
	if HasDataScope(authCtx, DataScopeShop, shopId) {
		return true
	}
	if HasDataScope(authCtx, DataScopeShopList, shopId) {
		return true
	}
	return false
}

// CanAccessUser 判断上下文身份能否访问指定用户。
// 允许本人、全部数据范围及匹配的个人范围；无身份或 userId 非正数时拒绝。
func CanAccessUser(ctx context.Context, userId int64) bool {
	if userId <= 0 {
		return false
	}
	authCtx, ok := CtxAuth(ctx)
	if !ok {
		return false
	}
	if authCtx.UID > 0 && authCtx.UID == userId {
		return true
	}
	if HasDataScope(authCtx, DataScopeAll, 0) {
		return true
	}
	if HasDataScope(authCtx, DataScopeSelf, userId) {
		return true
	}
	return false
}
