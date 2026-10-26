package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionData struct {
	UID          int64       `json:"uid"`
	ClientType   string      `json:"client_type"`
	RoleIds      []int64     `json:"role_ids"`
	PermCodes    []string    `json:"perm_codes"`
	RuntimePerms []string    `json:"runtime_perms"`
	DataScopes   []DataScope `json:"data_scopes"`
	ShopId       int64       `json:"shop_id"`
	TokenVersion int64       `json:"token_version"`
	IsSuperAdmin bool        `json:"is_super_admin"` // 是否为系统超级管理员
	IsShopAdmin  bool        `json:"is_shop_admin"`  // 是否为商户管理员
}

func NewSessionId() string {
	return uuid.NewString()
}

func BuildSessionKey(clientType, sid string) string {
	return fmt.Sprintf("sess:%s:%s", clientType, sid)
}

func BuildSessionIndexKey(clientType string, uid int64) string {
	return fmt.Sprintf("sess:%s:uid:%d", clientType, uid)
}

func SaveSession(ctx context.Context, redisClient *redis.Redis, sid string, data SessionData, expireSeconds int64) error {
	if redisClient == nil || sid == "" || data.UID <= 0 {
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
	if _, err := redisClient.SaddCtx(ctx, indexKey, sid); err != nil {
		return err
	}
	return redisClient.ExpireCtx(ctx, indexKey, int(expireSeconds))
}

func LoadSession(ctx context.Context, redisClient *redis.Redis, clientType, sid string) (*SessionData, error) {
	if redisClient == nil || clientType == "" || sid == "" {
		return nil, ErrSessionNotFound
	}
	key := BuildSessionKey(clientType, sid)
	payload, err := redisClient.GetCtx(ctx, key)
	if err != nil || payload == "" {
		return nil, ErrSessionNotFound
	}
	var data SessionData
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func RemoveSession(ctx context.Context, redisClient *redis.Redis, clientType, sid string, uid int64) error {
	if redisClient == nil || clientType == "" || sid == "" {
		return nil
	}
	key := BuildSessionKey(clientType, sid)
	_, _ = redisClient.DelCtx(ctx, key)
	if uid > 0 {
		indexKey := BuildSessionIndexKey(clientType, uid)
		_, _ = redisClient.SremCtx(ctx, indexKey, sid)
	}
	return nil
}

func RemoveUserSessions(ctx context.Context, redisClient *redis.Redis, clientType string, uid int64) error {
	if redisClient == nil || clientType == "" || uid <= 0 {
		return nil
	}
	indexKey := BuildSessionIndexKey(clientType, uid)
	sids, err := redisClient.SmembersCtx(ctx, indexKey)
	if err != nil {
		return err
	}
	for _, sid := range sids {
		key := BuildSessionKey(clientType, sid)
		_, _ = redisClient.DelCtx(ctx, key)
	}
	_, _ = redisClient.DelCtx(ctx, indexKey)
	return nil
}
