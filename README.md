# xinlin

供 Go/go-zero 项目复用的基础组件库，基于 `qingshanghui/server/base` 提取。模块路径为 `github.com/anheicang/xinlin`，保留 `base/...` 包结构和现有导出接口，便于从复制代码迁移到模块依赖。

## 安装

要求 Go 1.21.13 或以上版本。

```sh
go get github.com/anheicang/xinlin@v0.1.0
```

在业务项目中按需导入子包；`go get` 将组件作为依赖管理，源码由 Go module cache 管理，无需再复制到项目目录。如需将依赖源码随项目保存，可执行 `go mod vendor`。

```go
import (
    "github.com/anheicang/xinlin/base/baseerror"
    "github.com/anheicang/xinlin/base/baseresponse"
    "github.com/anheicang/xinlin/base/phone"
    "github.com/anheicang/xinlin/base/points"
)
```

## 组件

| 包路径（均位于 `base/` 下） | 能力 |
| --- | --- |
| `authentication` | JWT、认证上下文、权限与数据范围、Redis 会话及 token 版本 |
| `baseerror` | 业务错误码、HTTP 业务错误 |
| `baseresponse` | handler 统一成功、业务错误与系统异常响应 |
| `proto` | HTTP JSON 响应基础封装 |
| `phone` | 手机号空白字符清理与搜索值规范化 |
| `points` | 两位小数金额/积分与整数分转换、比例计算 |
| `secretbox` | AES-256-GCM 加密与用途绑定 |
| `safelog` | go-zero 安全访问日志和 SQL 参数日志关闭 |
| `kd100` | 快递100查询、签名与轨迹时间解析 |
| `aliyun/oss` | OSS 上传、STS 临时凭证和缓存 |
| `aliyun/sms` | 阿里云短信客户端 |
| `wx/wxapi` | 微信登录、手机号、订阅消息、内容检测及公众号接口 |

`phone`、`points`、`secretbox` 仅使用标准库；其他组件按需使用 go-zero 或供应商 SDK。模块只声明这些组件实际需要的依赖，不携带原服务入口、model、数据库或业务配置。

## 使用示例

```go
func handler(w http.ResponseWriter, r *http.Request) {
    amount, err := points.Parse("12.34") // 1234 分
    if err != nil {
        baseresponse.FormatResponseWithRequest(nil,
            baseerror.NewCodeError(40001, "金额格式不正确"), w, r)
        return
    }
    baseresponse.FormatResponseWithRequest(map[string]int64{"amount": amount}, nil, w, r)
}
```

成功返回 HTTP 200、`{"code":0,"data":...}`；普通业务错误返回 HTTP 406；`NewHttpCodeError` 可指定 HTTP 状态；未知异常返回 HTTP 500 及通用提示。响应协议沿用蓝本，接入时保持前端约定一致。

仓库中的可运行示例：

```sh
go run ./examples/basic
```

## 从复制代码迁移

1. 在业务项目执行上述 `go get`。
2. 将 `"api/base/xxx"` 替换为 `"github.com/anheicang/xinlin/base/xxx"`，包括子目录和测试中的 import。若旧 module 名不是 `api`，使用实际旧前缀。
3. 自定义业务错误码、权限码、短信模板和业务规则继续由业务项目维护。旧 base 有本地定制时，先比较差异，再选择公共实现或保留业务扩展。
4. 统一迁移所有使用认证上下文的中间件和 logic。旧包与新包的类型和私有 context key 不同，不可混用。
5. 执行 `go mod tidy`、`go test ./...` 和 `go build ./...`。确认无旧 import 后，再移除业务项目里重复的 base 实现。

本次提取未修改原 `qingshanghui` 项目。

## 接入约定

- 鉴权保留原项目的 `admin/shop/app` 客户端、商户/用户数据范围和 Redis key 约定；接入方需确认符合自己的权限模型。会话与 token 版本 key 没有项目级前缀，不同项目应使用独立 Redis 实例或独立 DB；微信 OAuth 缓存同样需要隔离。
- JWT 的 `GetJwtToken*` 沿用历史参数名 `iat`，调用时传入有效期秒数，如 `3600`；JWT claims 的历史格式也保留。库不自动安装鉴权中间件，由业务项目负责校验并注入 `AuthContext`。
- `phone.Normalize` 清理空白，不负责手机号有效性校验，也不删除 `+` 或 `-`。
- `points` 保留两位小数、非负解析和最大安全整数限制；业务计算使用整数分。
- `secretbox` 的 key 是 32 字节密钥的 Base64 编码；解密必须传入加密时相同的 purpose。密钥由调用方注入并保管。
- 云服务凭证和微信配置由调用方传入。微信订阅消息的 `Data` 保留现有模板字段，其他模板需由业务项目扩展。
- 使用 `safelog.Access` 时，应关闭 go-zero 默认请求日志（`Middlewares.Log = false`），通过 `safelog.Router` 包装路由，并注册 `server.Use(safelog.Access)`；`safelog.DisableSQLStatements()` 应在初始化数据库前调用。

## 开发与发布

```sh
gofmt -w .
go mod tidy
go test ./...
go vet ./...
go build ./...
```

GitHub Actions 自动检查格式、依赖一致性、静态分析、竞态测试及编译。普通测试使用本地 HTTP 服务或模拟数据，不需要真实云服务凭证。真实 STS 测试默认跳过，只有设置 `XINLIN_OSS_STS_INTEGRATION=1` 和 `XINLIN_OSS_CONFIG_FILE`（包含 `Oss` 节点的本地 YAML 路径）才会调用阿里云。

发布使用语义化版本标签，例如 `v0.1.0`；项目中固定版本升级，避免依赖浮动分支：

```sh
go get github.com/anheicang/xinlin@v0.1.0
go mod tidy
```

维护时在本仓库修改、测试并发布新标签，业务项目按需升级，无需再次复制源码。`v0.x` 阶段的接口变更应阅读对应提交和迁移说明；未来发布 `v2` 时使用 Go 规定的 `/v2` module 路径。
