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
	defaultTokenVersion int64 = 1

	tokenVersionKeySysUserFormat    = "jv_sys_user:%d"
	tokenVersionKeyShopUserFormat   = "jv_shop_sys_user:%d:%d"
	tokenVersionKeyMemberUserFormat = "jv_member_user:%d"
)

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
