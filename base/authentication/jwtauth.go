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
	AppXUserIdKey = "X-User-Id"
)

var (
	ErrUserInconsistentIdentity = baseerror.NewCodeError(90001, "用户调用身份不一致")
)

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

func GetJwtToken(secretKey string, iat, userId int64) (string, error) {
	return GetJwtTokenWithSession(secretKey, iat, userId, "", "")
}

// GetJwtTokenWithSession 生成包含 client_type + sid 的 JWT
func GetJwtTokenWithSession(secretKey string, iat, userId int64, clientType, sid string) (string, error) {
	seconds := time.Now().Unix()
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = iat
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

// Deprecated: use GetJwtTokenWithSession instead
func GetJwtTokenWithPerms(secretKey string, iat, userId int64, perms []string) (string, error) {
	seconds := time.Now().Unix()
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = iat
	claims["uid"] = userId
	claims["perms"] = perms
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	return token.SignedString([]byte(secretKey))
}

func ParseToken(tokenString, secretKey string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	key := []byte(secretKey)
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return key, nil
	})

	if err != nil {
		return nil, err
	}

	if token.Valid {
		return claims, nil
	} else {
		return nil, errors.New("invalid token")
	}
}

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
