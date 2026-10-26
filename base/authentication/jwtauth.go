// 文件用途：处理 JWT、Authorization 请求头与用户身份匹配，供业务认证中间件调用。

package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anheicang/xinlin/base/baseerror"
	"github.com/anheicang/xinlin/base/baseresponse"

	"github.com/golang-jwt/jwt/v4"
)

const (
	// AppXUserIdKey 是旧请求协议声明用户 ID 的 Header 名；不能代替可信认证结果。
	AppXUserIdKey = "X-User-Id"
)

var (
	// ErrUserInconsistentIdentity 表示请求声明的身份或 token 与可信上下文不一致。
	ErrUserInconsistentIdentity = baseerror.NewCodeError(90001, "用户调用身份不一致")
)

// GetHttpRequestToken 读取 Authorization token，并比较可信上下文 userId 与 X-User-Id 请求头。
// 第二个返回值仅表示身份编号一致；本函数不校验 JWT 签名，调用前需完成认证。
func GetHttpRequestToken(r *http.Request) (string, bool, error) {
	token := r.Header.Get("Authorization")
	token = strings.Replace(token, "Bearer ", "", 1)
	userIdValue := r.Context().Value(ctxKeyUserId)
	if userIdValue == nil {
		userIdValue = r.Context().Value("userId")
	}
	if userIdValue == nil {
		return "", false, errors.New("userId is empty")
	}
	var userId string
	switch v := userIdValue.(type) {
	case json.Number:
		userId = v.String()
	case int64:
		userId = fmt.Sprintf("%d", v)
	case float64:
		userId = fmt.Sprintf("%d", int64(v))
	case string:
		userId = v
	default:
		return "", false, errors.New("userId is invalid")
	}
	xUserId := r.Header.Get(AppXUserIdKey)
	return token, userId == xUserId, nil
}

// Validate 比较请求 token、传入 token 和请求声明的用户身份。
// 校验失败时写入 HTTP 400 或 401 并返回 false；应在已完成认证的请求流程中调用。
func Validate(reqToken string, r *http.Request, w http.ResponseWriter) bool {
	token, flag, err := GetHttpRequestToken(r)
	if err != nil {
		baseresponse.HttpParamErrorWithRequest(w, r, err)
		return false
	} else if !flag {
		baseresponse.HttpUnauthorized(w, ErrUserInconsistentIdentity)
		return false
	} else if reqToken != token {
		baseresponse.HttpUnauthorized(w, ErrUserInconsistentIdentity)
		return false
	}
	return true
}

// GetJwtToken 签发只携带用户 ID 的 HS256 JWT，是旧调用方式的便捷入口。
// 历史参数 iat 实际表示有效期秒数，例如 3600；secretKey 由调用方安全注入。
func GetJwtToken(secretKey string, iat, userId int64) (string, error) {
	return GetJwtTokenWithSession(secretKey, iat, userId, "", "")
}

// GetJwtTokenWithSession 签发包含用户 ID、可选客户端标识和会话 ID 的 HS256 JWT。
// iat 参数表示有效期秒数，生成的 iat claim 为当前时间，exp 为当前时间加有效期。
func GetJwtTokenWithSession(secretKey string, iat, userId int64, clientType, sid string) (string, error) {
	seconds := time.Now().Unix()
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = seconds
	claims["uid"] = userId
	if clientType != "" {
		claims["client_type"] = clientType
	}
	if sid != "" {
		claims["sid"] = sid
	}
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	return token.SignedString([]byte(secretKey))
}

// GetJwtTokenWithPerms 签发直接携带权限码的旧版 JWT。
// iat 参数仍为有效期秒数；权限变更不会自动更新已签发的 token。
//
// Deprecated: use GetJwtTokenWithSession instead
func GetJwtTokenWithPerms(secretKey string, iat, userId int64, perms []string) (string, error) {
	seconds := time.Now().Unix()
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = seconds
	claims["uid"] = userId
	claims["perms"] = perms
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	return token.SignedString([]byte(secretKey))
}

// ParseToken 校验 JWT 签名和库内置的时间约束，并返回 claims。
// 数字字段使用 json.Number 保留 int64 精度；本函数不检查 Redis 会话或业务权限。
func ParseToken(tokenString, secretKey string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	key := []byte(secretKey)
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return key, nil
	}, jwt.WithJSONNumber())

	if err != nil {
		return nil, err
	}

	if token.Valid {
		return claims, nil
	} else {
		return nil, errors.New("invalid token")
	}
}

// GetUserId 从请求的 Authorization JWT 中读取 uid。
// 签名错误、uid 缺失或整数解析失败时返回错误，支持大整数用户 ID。
func GetUserId(req *http.Request, secretKey string) (int64, error) {
	token := req.Header.Get("Authorization")
	token = strings.Replace(token, "Bearer ", "", 1)
	claims, err := ParseToken(token, secretKey)
	if err != nil {
		return 0, err
	}

	userIdTemp, ok := claims["uid"]
	if !ok {
		return 0, errors.New("不存在uid字段")
	}
	switch v := userIdTemp.(type) {
	case float64:
		return int64(v), nil
	case json.Number:
		id, err := v.Int64()
		if err != nil {
			return 0, err
		}
		return id, nil
	case int64:
		return v, nil
	default:
		return 0, errors.New("uid类型不正确")
	}
}

// GetPerms 从已签名的 JWT 中读取历史 perms 字段。
// 兼容字符串和字符串数组；缺失或不支持的值返回空列表，不替代服务端权限校验。
func GetPerms(req *http.Request, secretKey string) ([]string, error) {
	token := req.Header.Get("Authorization")
	token = strings.Replace(token, "Bearer ", "", 1)
	claims, err := ParseToken(token, secretKey)
	if err != nil {
		return nil, err
	}

	rawPerms, ok := claims["perms"]
	if !ok || rawPerms == nil {
		return []string{}, nil
	}

	switch value := rawPerms.(type) {
	case []string:
		return value, nil
	case []interface{}:
		perms := make([]string, 0, len(value))
		for _, item := range value {
			if item == nil {
				continue
			}
			if str, ok := item.(string); ok {
				perms = append(perms, str)
			}
		}
		return perms, nil
	case string:
		if value == "" {
			return []string{}, nil
		}
		return []string{value}, nil
	default:
		return []string{}, nil
	}
}

// MustUserId 为旧流程尝试补充用户 ID 和 token 权限。
// 已有 AuthContext 时直接复用；解析失败时保留原 context，调用方仍须负责拒绝未认证请求。
func MustUserId(ctx context.Context, req *http.Request, secretKey string) context.Context {
	if _, ok := CtxAuth(ctx); ok {
		return ctx
	}
	userId, err := GetUserId(req, secretKey)
	if err != nil {
		return ctx
	}
	next := context.WithValue(ctx, ctxKeyUserId, userId)
	perms, err := GetPerms(req, secretKey)
	if err != nil {
		return next
	}
	return context.WithValue(next, ctxKeyPerms, perms)
}
