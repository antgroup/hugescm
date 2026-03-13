# Keyring - 跨平台密钥管理库

基于 purego 的跨平台密钥管理库，完全兼容 git credential 工具。

## Linux 特殊说明

**重要：Linux 下的默认行为**

在 Linux 系统上，keyring 默认**不存储密码**，以避免在无 GUI 或服务器环境中出现 DBUS 连接错误。

### 启用密码存储

如需在 Linux 上启用密码存储，有以下两种方式：

#### 1. 使用环境变量（推荐用于 CI/CD 或临时使用）

```bash
export ZETA_CREDENTIAL_STORAGE=secret-service
```

#### 2. 使用配置文件（推荐用于长期使用）

```bash
# 全局配置
zeta config --global credential.storage secret-service

# 或本地配置
zeta config credential.storage secret-service
```

### 存储模式说明

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `auto` | 自动选择（默认） | 自动检测环境，Linux 下默认不存储 |
| `secret-service` | 使用 libsecret/Secret Service | 有桌面环境的 Linux（需要 DBUS） |
| `none` | 禁用存储 | 完全禁用凭据存储 |

## 与 zalando/go-keyring 的重大差异

### 1. 完全兼容 Git Credential 工具

- **go-keyring**: 使用自定义的查询和存储格式，与 git credential 不兼容
- **zeta/keyring**: 严格按照 git credential 工具的格式和属性存储凭据

**兼容的工具：**
- `git-credential-osxkeychain` (macOS)
- `git-credential-manager` (Windows)
- `git-credential-libsecret` (Linux)

### 2. 纯 Purego 实现

- **go-keyring**: macOS 使用 cgo 调用 Security framework，Windows 使用 syscall
- **zeta/keyring**: 完全使用 purego，通过纯 Go 代码调用平台 API

**优点：**
- 无 CGO 依赖，编译更简单
- 支持交叉编译
- 更好的可移植性

### 3. 统一的凭据结构

- **go-keyring**: 使用简单的 `(service, username, password)` 三元组
- **zeta/keyring**: 使用完整的凭据结构，包含 protocol、server、path、port 等信息

```go
type Cred struct {
    UserName string
    Password string
    Protocol string // 协议类型：http, https, imap, smtp, ftp 等
    Server   string // 服务器地址（不含端口）
    Path     string // 路径（可选）
    Port     int    // 端口（可选）
}
```

### 4. 函数命名符合 Git 惯例

- **go-keyring**: 使用 `Get/Set/Delete`
- **zeta/keyring**: 使用 `Get/Store/Erase`，与 git credential 的 `get/store/erase` 命令保持一致

### 5. 多用户支持

- **go-keyring**: 一个 service 只能有一个 username
- **zeta/keyring**: 同一 server 可以有多个不同的 username，完全支持多用户场景

### 6. 移除接口抽象

- **go-keyring**: 定义了 `Keyring` 接口和多种实现
- **zeta/keyring**: 直接导出平台特定的函数，通过 build tags 选择实现

**优点：**
- 代码更简洁，减少抽象层次
- 调用方更直观，无需实例化对象

## 使用方式

### 基本用法

```go
import "github.com/zeta/zeta/modules/keyring"

// 从 URL 解析凭据
cred := keyring.NewCredFromURL("https://github.com/zeta/zeta")

// 设置密码
cred.UserName = "username"
cred.Password = "password"

// 存储
err := keyring.Store(context.Background(), cred)

// 获取
retrieved, err := keyring.Get(context.Background(), cred)
if err == nil {
    fmt.Println("Password:", retrieved.Password)
}

// 删除
err := keyring.Erase(context.Background(), cred)
```

### 从 URL 自动解析

```go
// 支持多种 URL 格式
cred1 := keyring.NewCredFromURL("https://github.com/zeta/zeta")
// cred1.Protocol = "https"
// cred1.Server = "github.com"

cred2 := keyring.NewCredFromURL("http://example.com:8080/path")
// cred2.Protocol = "http"
// cred2.Server = "example.com"
// cred2.Port = 8080
// cred2.Path = "/path"
```

### 手动构造凭据

```go
cred := &keyring.Cred{
    Protocol: "https",
    Server:   "example.com",
    Port:     443,
    UserName: "user",
    Password: "pass",
}

err := keyring.Store(context.Background(), cred)
```

## 平台实现

### macOS (Darwin)

- 使用 Security framework
- 完全兼容 `git-credential-osxkeychain`
- 纯 purego 实现，无 CGO 依赖
- 支持：kSecAttrProtocol、kSecAttrAuthenticationType 等属性

**目标名称格式：** `server[:port]`

### Windows

- 使用 Windows Credential Manager API
- 完全兼容 `git-credential-manager`
- 支持 UTF-16 编码

**目标名称格式：** `zeta:<protocol>:<server>[:<port>][<path>]`

### Linux/Unix

- **默认行为**：不存储密码，避免 DBUS 错误
- 可选使用 Secret Service API (libsecret)
- 完全兼容 `git-credential-libsecret`
- 需要显式配置才能启用存储

**启用存储：**

```bash
# 方式1：环境变量
export ZETA_CREDENTIAL_STORAGE=secret-service

# 方式2：配置文件
zeta config credential.storage secret-service
```

**目标名称格式：** `zeta:<protocol>:<server>[:<port>][<path>]`

## 错误处理

```go
cred := keyring.NewCredFromURL("https://example.com")

// 检查凭据是否存在
_, err := keyring.Get(context.Background(), cred)
if errors.Is(err, keyring.ErrNotFound) {
    fmt.Println("Credential not found")
}

// 检查存储是否被禁用（Linux 默认行为）
err = keyring.Store(context.Background(), cred)
if errors.Is(err, keyring.ErrStorageDisabled) {
    fmt.Println("Credential storage is disabled on Linux")
    fmt.Println("To enable: export ZETA_CREDENTIAL_STORAGE=secret-service")
}
```

## 最佳实践

1. **始终使用完整的凭据信息**：包括 protocol、server、username 等
2. **使用 NewCredFromURL**：从 URL 自动解析，避免手动构造错误
3. **处理 ErrNotFound**：区分"找不到"和"其他错误"
4. **处理 ErrStorageDisabled**：在 Linux 上检查存储是否启用
5. **使用 context**：支持超时和取消操作
6. **不要硬编码密码**：始终使用 keyring 存储敏感信息
7. **Linux 环境**：明确告知用户如何启用凭据存储

## 限制

- 每个凭据必须有 server 字段
- Username 和 Password 不能为空
- 不支持空字节（null byte）在这些字段中

## 许可证

Apache License Version 2.0