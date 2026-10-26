// 文件用途：维护登录态版本，使业务项目能通过递增版本撤销旧 token。

package authentication

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// 默认登录版本从 1 开始，撤销时必须递增到更大的值。
	defaultTokenVersion int64 = 1

	tokenVersionKeySysUserFormat    = "jv_sys_user:%d"
	tokenVersionKeyShopUserFormat   = "jv_shop_sys_user:%d:%d"
	tokenVersionKeyMemberUserFormat = "jv_member_user:%d"
)

// buildTokenVersionKey 按客户端、商户和用户构造登录版本 key。
// app 用户使用平台级 key，不按商户拆分；非法客户端或 ID 返回错误。
func buildTokenVersionKey(clientType string, shopId, uid int64) (string, error) {
	if uid <= 0 {
		return "", errors.New("invalid token version uid")
	}
	switch clientType {
	case ClientTypeAdmin:
		return fmt.Sprintf(tokenVersionKeySysUserFormat, uid), nil
	case ClientTypeShop:
		if shopId < 0 {
			return "", errors.New("invalid token version shopId")
		}
		return fmt.Sprintf(tokenVersionKeyShopUserFormat, shopId, uid), nil
	case ClientTypeApp:
		// 小程序会员属于平台会员，不按商户隔离登录态。
		return fmt.Sprintf(tokenVersionKeyMemberUserFormat, uid), nil
	default:
		return "", errors.New("invalid token version clientType")
	}
}

// GetTokenVersion 读取当前登录版本，缺失时以默认版本 1 初始化。
// 业务中间件可将会话中的版本与此结果比较，以拒绝已撤销的登录。
func GetTokenVersion(ctx context.Context, redisClient *redis.Redis, clientType string, shopId, uid int64) (int64, error) {
	if redisClient == nil || uid <= 0 {
		err := errors.New("invalid token version args")
		logx.Errorf("GetTokenVersion invalid args uid=%d shopId=%d clientType=%s redisNil=%v", uid, shopId, clientType, redisClient == nil)
		return 0, err
	}

	key, err := buildTokenVersionKey(clientType, shopId, uid)
	if err != nil {
		logx.Errorf("GetTokenVersion buildTokenVersionKey error uid=%d shopId=%d clientType=%s err=%v", uid, shopId, clientType, err)
		return 0, err
	}
	val, err := redisClient.GetCtx(ctx, key)
	if err != nil {
		logx.Errorf("GetTokenVersion RedisClient.GetCtx error uid=%d shopId=%d clientType=%s err=%v", uid, shopId, clientType, err)
		return 0, err
	}

	if val == "" {
		if _, err := redisClient.SetnxCtx(ctx, key, strconv.FormatInt(defaultTokenVersion, 10)); err != nil {
			logx.Errorf("GetTokenVersion RedisClient.SetnxCtx error uid=%d shopId=%d clientType=%s err=%v", uid, shopId, clientType, err)
			return 0, err
		}
		val, err = redisClient.GetCtx(ctx, key)
		if err != nil {
			logx.Errorf("GetTokenVersion RedisClient.GetCtx error uid=%d shopId=%d clientType=%s err=%v", uid, shopId, clientType, err)
			return 0, err
		}
		if val == "" {
			return defaultTokenVersion, nil
		}
	}

	version, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		logx.Errorf("GetTokenVersion parse error uid=%d shopId=%d clientType=%s val=%s err=%v", uid, shopId, clientType, val, err)
		return 0, err
	}
	if version <= 0 {
		err := errors.New("invalid token version value")
		logx.Errorf("GetTokenVersion invalid value uid=%d shopId=%d clientType=%s val=%d", uid, shopId, clientType, version)
		return 0, err
	}
	return version, nil
}

// IncrTokenVersion 原子递增登录版本，使旧版本 token/会话失效。
// key 缺失时先初始化为 1 再递增，首次撤销返回 2；业务方需实际执行版本比对。
func IncrTokenVersion(ctx context.Context, redisClient *redis.Redis, clientType string, shopId, uid int64) (int64, error) {
	if redisClient == nil || uid <= 0 {
		err := errors.New("invalid token version args")
		logx.Errorf("IncrTokenVersion invalid args uid=%d shopId=%d clientType=%s redisNil=%v", uid, shopId, clientType, redisClient == nil)
		return 0, err
	}

	key, err := buildTokenVersionKey(clientType, shopId, uid)
	if err != nil {
		logx.Errorf("IncrTokenVersion buildTokenVersionKey error uid=%d shopId=%d clientType=%s err=%v", uid, shopId, clientType, err)
		return 0, err
	}
	// 初始化与递增放在同一个 Lua 请求中，避免缺失 key 时 INCR 只产生仍有效的版本 1。
	result, err := redisClient.EvalCtx(ctx, `
redis.call('SET', KEYS[1], ARGV[1], 'NX')
return redis.call('INCR', KEYS[1])`, []string{key}, defaultTokenVersion)
	if err != nil {
		logx.Errorf("IncrTokenVersion RedisClient.IncrCtx error uid=%d shopId=%d clientType=%s err=%v", uid, shopId, clientType, err)
		return 0, err
	}
	version, ok := result.(int64)
	if !ok || version <= defaultTokenVersion {
		return 0, errors.New("invalid token version result")
	}
	return version, nil
}
