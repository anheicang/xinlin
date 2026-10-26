// Package safelog 为 go-zero 提供不记录敏感请求内容的访问日志和 SQL 语句日志关闭入口。
// 使用 Access 时应关闭框架默认请求日志，并使用 Router 注入路由模板，避免记录路径中的业务标识。
package safelog
