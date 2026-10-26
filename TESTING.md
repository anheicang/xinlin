# 单元测试

所有 12 个基础组件包均有单元测试，所有可执行函数均被测试调用。默认测试不依赖真实 Redis、MySQL、阿里云或微信账号，不执行付费操作。

## 运行

在仓库根目录执行（PowerShell 中包含点号的 Go 参数建议保留引号）：

```sh
go test ./... "-coverprofile=coverage.out"
go tool cover "-func=coverage.out"
go tool cover "-html=coverage.out" "-o=coverage.html"
go vet ./...
go build ./...
```

随机顺序检查与竞态检测：

```sh
go test -shuffle=on ./...
go test -race ./...
```

竞态检测要求支持 CGO 的 Go 环境和 C 编译器。GitHub Actions 在 Ubuntu 执行竞态检测，检查总语句覆盖率至少 95%、不存在覆盖率为 0 的函数，并上传文本与 HTML 覆盖率报告。

## 当前覆盖率

本次全量运行的总语句覆盖率为 **98.1%**（包含示例程序）。

| 包 | 语句覆盖率 | 主要验证内容 |
| --- | --- | --- |
| base/aliyun/oss | 99.1% | 上传、下载、元信息、Bucket、签名 URL、STS、缓存刷新、并发和失败 |
| base/aliyun/sms | 91.7% | SDK 请求初始化、HTTPS、模板参数、业务错误、网络错误、无效消息 |
| base/authentication | 95.8% | JWT、大整数 ID、过期与签名、身份匹配、权限、数据范围、会话与版本 |
| base/baseerror | 100.0% | 错误字段、错误链、HTTP 状态、空错误、批处理短路 |
| base/baseresponse | 100.0% | 成功、业务错误、异常、HTTP 状态、错误详情、旧接口与辅助函数 |
| base/kd100 | 97.5% | 请求签名、参数校验、状态解析、轨迹时间、网络和响应异常 |
| base/phone | 100.0% | Unicode 空白、搜索字符串、保留有效字符 |
| base/points | 100.0% | 金额格式、精度、最大值、负值显示、比例和溢出边界 |
| base/proto | 100.0% | JSON 字段、HTTP 状态及 Content-Type |
| base/safelog | 100.0% | 日志脱敏、panic、流式响应、CORS、可选 Writer 接口和 SQL 日志 |
| base/secretbox | 95.5% | 加解密、随机 nonce、用途绑定、密钥、篡改、非法密文和熵源错误 |
| base/wx/wxapi | 100.0% | 登录、OAuth、手机号、缓存刷新、订阅、内容检测及公众号异常 |

模块根目录不包含 Go 包，调用方按需导入 base 下的子包；示例程序也有输出验证。结构体、常量、接口声明不产生覆盖率语句。

剩余未覆盖部分主要是防御性分支：例如固定 32 字节密钥通过校验后 AES 构造器报错、仅含字符串的固定请求结构无法 JSON 序列化、SDK 成功时返回空响应、JWT JSON Number 模式下遗留的其他数字类型分支，以及示例固定有效金额解析失败。测试不会通过改坏运行时代码来制造这些状态，语句覆盖率也不代表所有输入组合都已穷尽。

## 测试隔离

- Redis 使用 miniredis，验证 TTL、故障、Lua 脚本和并发递增。
- 云接口使用 SDK 工厂替换或 HTTP RoundTripper，断言实际请求与响应，阻止真实外部请求。
- 需要替换包内工厂、日志输出或随机源的测试不调用 t.Parallel，并通过 Cleanup 恢复状态；STS 并发测试会等待全部 goroutine 结束。
- 默认跳过 TestGetOssSts 的真实 STS 集成测试；只有显式设置 XINLIN_OSS_STS_INTEGRATION=1 与 XINLIN_OSS_CONFIG_FILE 才会执行。

coverage.out、coverage.txt 和 coverage.html 是本地/CI 产物，不提交到 Git。
