// Package authentication 提供 JWT 签发与解析、请求身份上下文、权限和数据范围判断，以及 Redis 会话管理。
// 认证中间件应在可信身份校验后注入 AuthContext；不同项目使用独立 Redis 实例或 DB 隔离登录态。
package authentication
