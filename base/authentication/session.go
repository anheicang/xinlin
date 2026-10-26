// 文件用途：保存、读取和撤销 Redis 登录会话，维护用户到会话 ID 的索引。

package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// ErrSessionNotFound 表示会话不存在，或无法构成有效的会话查询参数。
// Redis 连接/读取故障会返回原始错误，调用方应区分基础设施异常与正常未登录。
var ErrSessionNotFound = errors.New("session not found")

// SessionData 定义保存到 Redis 的服务端登录快照，不是直接返回前端的 DTO。
type SessionData struct {
	UID          int64       `json:"uid"`            // 当前用户 ID，使用 int64 保存。
	ClientType   string      `json:"client_type"`    // 客户端类别：admin、shop 或 app。
	RoleIds      []int64     `json:"role_ids"`       // 角色 ID 列表，具体角色含义由业务项目定义。
	PermCodes    []string    `json:"perm_codes"`     // 会话中保存的角色/持久权限码。
	RuntimePerms []string    `json:"runtime_perms"`  // 会话中保存的运行时权限码，由业务认证流程解释。
	DataScopes   []DataScope `json:"data_scopes"`    // 服务端授予的数据访问范围。
	ShopId       int64       `json:"shop_id"`        // 所属商户 ID；没有商户身份时为 0。
	TokenVersion int64       `json:"token_version"`  // 登录版本快照，需与 Redis 当前版本比较以判断是否已撤销。
	IsSuperAdmin bool        `json:"is_super_admin"` // 是否为系统超级管理员
	IsShopAdmin  bool        `json:"is_shop_admin"`  // 是否为商户管理员
}

// NewSessionId 生成随机 UUID 会话 ID，用于 JWT 的 sid 和 Redis 会话 key。
func NewSessionId() string {
	return uuid.NewString()
}

// BuildSessionKey 构造按客户端和会话 ID 隔离的 Redis 会话 key。
// 不同项目仍需通过 Redis 实例或 DB 隔离，key 中不包含项目级前缀。
func BuildSessionKey(clientType, sid string) string {
	return fmt.Sprintf("sess:%s:%s", clientType, sid)
}

// BuildSessionIndexKey 构造用户到会话 ID 集合的索引 key，用于批量撤销同端登录。
func BuildSessionIndexKey(clientType string, uid int64) string {
	return fmt.Sprintf("sess:%s:uid:%d", clientType, uid)
}

// SaveSession 保存会话数据，并把 sid 加入该用户的会话索引。
// expireSeconds 为正整数秒；单 key Lua 原子更新索引且不缩短 TTL，索引失败时尝试删除新会话并返回错误。
func SaveSession(ctx context.Context, redisClient *redis.Redis, sid string, data SessionData, expireSeconds int64) error {
	if redisClient == nil || sid == "" || data.UID <= 0 || data.ClientType == "" || expireSeconds <= 0 {
		return errors.New("invalid session args")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	key := BuildSessionKey(data.ClientType, sid)
	if err := redisClient.SetexCtx(ctx, key, string(payload), int(expireSeconds)); err != nil {
		return err
	}
	indexKey := BuildSessionIndexKey(data.ClientType, data.UID)
	// 原子更新索引成员并仅延长 TTL，防止短会话使长会话提前失去撤销索引。
	// 脚本只操作一个 key，保留 Redis Cluster 兼容性；旧索引无过期时间时保持原状态。
	_, err = redisClient.EvalCtx(ctx, `
local ttl = redis.call('PTTL', KEYS[1])
redis.call('SADD', KEYS[1], ARGV[1])
local requested = tonumber(ARGV[2]) * 1000
if ttl ~= -1 and ttl < requested then
    redis.call('PEXPIRE', KEYS[1], requested)
end
return 1`, []string{indexKey}, sid, expireSeconds)
	if err != nil {
		_, cleanupErr := redisClient.DelCtx(ctx, key)
		return errors.Join(fmt.Errorf("index session: %w", err), cleanupErr)
	}
	return nil
}

// LoadSession 按客户端和 sid 读取会话并反序列化。
// 缺失时返回 ErrSessionNotFound；Redis 故障和 JSON 损坏返回原错误，便于调用方区分。
func LoadSession(ctx context.Context, redisClient *redis.Redis, clientType, sid string) (*SessionData, error) {
	if redisClient == nil || clientType == "" || sid == "" {
		return nil, ErrSessionNotFound
	}
	key := BuildSessionKey(clientType, sid)
	payload, err := redisClient.GetCtx(ctx, key)
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return nil, ErrSessionNotFound
	}
	var data SessionData
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// RemoveSession 删除单个会话，uid 大于 0 时同步移除用户索引成员。
// Redis 操作失败会返回错误；uid 为 0 时仅删除会话本身。
func RemoveSession(ctx context.Context, redisClient *redis.Redis, clientType, sid string, uid int64) error {
	if redisClient == nil || clientType == "" || sid == "" {
		return errors.New("invalid session args")
	}
	key := BuildSessionKey(clientType, sid)
	if _, err := redisClient.DelCtx(ctx, key); err != nil {
		return err
	}
	if uid > 0 {
		indexKey := BuildSessionIndexKey(clientType, uid)
		if _, err := redisClient.SremCtx(ctx, indexKey, sid); err != nil {
			return err
		}
	}
	return nil
}

// RemoveUserSessions 撤销当前索引快照中的该用户所有同端会话。
// 删除失败即返回并保留待重试索引；不会删除并发新登录加入的其他索引项。
func RemoveUserSessions(ctx context.Context, redisClient *redis.Redis, clientType string, uid int64) error {
	if redisClient == nil || clientType == "" || uid <= 0 {
		return errors.New("invalid session args")
	}
	indexKey := BuildSessionIndexKey(clientType, uid)
	sids, err := redisClient.SmembersCtx(ctx, indexKey)
	if err != nil {
		return err
	}
	for _, sid := range sids {
		key := BuildSessionKey(clientType, sid)
		if _, err := redisClient.DelCtx(ctx, key); err != nil {
			return err
		}
		if _, err := redisClient.SremCtx(ctx, indexKey, sid); err != nil {
			return err
		}
	}
	// 只移除快照中已处理的成员，避免删掉并发新登录刚加入的索引。
	return nil
}
