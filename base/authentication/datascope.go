package authentication

import "context"

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
