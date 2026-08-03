# zeta-serve 私有化部署指南

zeta-serve 是 HugeSCM 的服务端组件，提供完整的**云原生版本控制后端**，包含 HTTP API、Web 管理界面和 SSH 协议服务。它允许组织或个人在自有基础设施上进行私有化部署，无需二次开发即可获得一个开箱即用的版本控制平台。

---

## 一、架构概览

```
  zeta client                                    生产网络
     │
     ▼  HTTPS 443 / SSH 22
┌──────────────────────────────────────────────────────────┐
│                Nginx / Caddy（反向代理）                   │
│  -- TLS 证书终止 (Let's Encrypt / 企业 CA)                │
│  -- 443 → 127.0.0.1:21000  22 → 127.0.0.1:22000        │
└────────────────────┬─────────────────────────────────────┘
                     │
     ┌───────────────┴── HTTP (port 21000)                  SSH (port 22000)
     │                                                            │
     ▼                                                            ▼
 zeta-serve httpd                                          zeta-serve sshd
  ├── Z1 Wire Protocol (push/pull/checkout)                  ├── ls-remote (fetch references)
  ├── REST API  (/api/v1)                                   ├── metadata  (fetch commit/tree)
  └── Web UI     (/login, /repos, ...)                      ├── objects   (batch download/upload)
                                                            └── push      (push commit/refs)
                    ┌──────────────────┬──────────────────┐
                    │                  │                  │
              ┌─────┴─────┐    ┌───────┴──────┐   ┌──────┴───────┐
              │ MySQL/OB  │    │ 对象存储 OSS  │   │ 本地文件系统  │
              │ 元数据 DB │    │ Blob 内容存储  │   │ 仓库索引缓存  │
              │ commits   │    │ (大文件分片)   │   │              │
              │ trees     │    │               │   │              │
              │ branches  │    │               │   │              │
              │ users     │    │               │   │              │
              │ members   │    │               │   │              │
              └───────────┘    └──────────────┘   └──────────────┘
```

### 组件构成

| 组件 | 包路径 | 说明 |
|------|--------|------|
| HTTP Server (httpd) | `pkg/serve/httpserver` | HTTP 服务，整合 Z1 协议、REST API 和 Web 管理界面 |
| SSH Server (sshd) | `pkg/serve/sshserver` | SSH 服务，提供 zeta 协议命令 |
| Database | `pkg/serve/database` | MySQL/OceanBase 元数据存储层 |
| ODB (Object DB) | `pkg/serve/odb` | 对象存储抽象层，桥接元数据 DB 与 OSS |
| Repository Hub | `pkg/serve/repo` | 仓库生命周期管理（创建、打开） |

---

## 二、快速部署

### 2.1 前置条件

| 依赖 | 说明 |
|------|------|
| MySQL 5.7+ / OceanBase | 元数据存储，需创建数据库 `zetadev` |
| 兼容 S3 的对象存储 | Blob 内容存储（MinIO、阿里云 OSS、AWS S3 等） |
| Go 1.21+ | 构建工具链 |
| 反向代理（Nginx / Caddy） | TLS 证书终止、端口映射、负载均衡（生产环境必需） |

> 数据库 schema 位于 `pkg/serve/database/zeta.sql`，部署时直接导入。

### 2.2 构建

```shell
# 构建 zeta-serve 二进制
make zeta-serve

# 构建产物在 bin/zeta-serve
# 同时确保已构建 zeta 客户端
make build
```

### 2.3 生成密钥

zeta-serve 需要两类密钥：

#### SSH 主机密钥（sshd 专用）

```shell
# 生成 RSA 密钥
zeta-serve keygen -t RSA -bitSize 2048  > host_rsa_key
zeta-serve keygen -t ED25519              > host_ed25519_key
```

#### 配置加密切钥（X25519，用于加密配置中的密码）

```shell
# 生成 X25519 私钥
zeta-serve keygen -t X25519 > x25519_key.pem

# 加密数据库密码
zeta-serve encrypt "your_db_password" -c config.toml

# 也可从环境变量或文件读取明文
zeta-serve encrypt DB_PASSWORD -c config.toml -s   # 从环境变量
zeta-serve encrypt /path/to/secret -c config.toml -p  # 从文件
```

加密后的密文以 `ENC@` 前缀标识，服务启动时自动解密。

### 2.4 初始化数据库

```shell
# 创建数据库
mysql -h <host> -P <port> -u <user> -p -e "CREATE DATABASE zetadev DEFAULT CHARSET utf8mb4"

# 导入 schema
mysql -h <host> -P <port> -u <user> -p zetadev < pkg/serve/database/zeta.sql
```

### 2.5 编写配置文件

#### HTTP Server 配置 (`zeta-serve-httpd.toml`)

```toml
listen = "0.0.0.0:21000"
repositories = "/data/zeta/repositories"
banner_version = "zeta-serve/1.0"
x25519_key = '''
-----BEGIN PRIVATE KEY-----
<your x25519 key content>
-----END PRIVATE KEY-----
'''

[database]
name = "zetadev"
user = "zeta"
host = "127.0.0.1"
port = 2883
passwd = "ENC@<encrypted_password>"   # 可选：用 zeta-serve encrypt 加密

[oss]
endpoint = "https://oss.example.com"
shared_endpoint = "https://oss-direct.example.com"  # 直连下载端点（可选）
bucket = "zeta-blobs"
access_key_id = "ENC@<encrypted_key>"       # 可选：加密
access_key_secret = "ENC@<encrypted_secret>" # 可选：加密
region = "us-east-1"

[admin]  # 可选：首个管理员账户
username = "admin"
password = "ENC@<encrypted_password>"  # 配置 x25519_key 后可加密
email = "admin@example.com"

[cache]
num_counters = 1000000000
max_cost = 20
buffer_items = 64
```

#### SSH Server 配置 (`zeta-serve-sshd.toml`)

```toml
listen = "0.0.0.0:22000"
endpoint = "zeta.example.com"       # 客户端连接用的 host（用于生成 remote URL）
repositories = "/data/zeta/repositories"
banner_version = "zeta-serve/1.0"
x25519_key = '''
-----BEGIN PRIVATE KEY-----
<your x25519 key content>
-----END PRIVATE KEY-----
'''
host_private_keys = [
    '''-----BEGIN RSA PRIVATE KEY-----
<host key content>
-----END RSA PRIVATE KEY-----''',
    '''-----BEGIN PRIVATE KEY-----
<ed25519 key content>
-----END PRIVATE KEY-----''',
]

[database]
name = "zetadev"
user = "zeta"
host = "127.0.0.1"
port = 2883
passwd = "ENC@<encrypted_password>"

[oss]
endpoint = "https://oss.example.com"
shared_endpoint = "https://oss-direct.example.com"
bucket = "zeta-blobs"
access_key_id = "ENC@<encrypted_key>"
access_key_secret = "ENC@<encrypted_secret>"

[cache]
num_counters = 1000000000
max_cost = 20
buffer_items = 64
```

> 配置文件支持环境变量展开。使用 `-E` 标志启动时，`$${var}` 和 `$$var` 会被替换为对应环境变量的值：
> ```shell
> zeta-serve httpd -c config.toml -E
> ```

### 2.6 启动服务

```shell
# 启动 HTTP 服务
zeta-serve httpd -c ~/config/zeta-serve-httpd.toml

# 启动 SSH 服务
zeta-serve sshd -c ~/config/zeta-serve-sshd.toml

# 调试模式
zeta-serve httpd -c config.toml -V   # verbose
zeta-serve sshd -c config.toml -V
```

启动后若配置了 `[admin]` 段，首个管理员账号会自动创建（幂等，重复启动不会覆盖已有账号）。

---

## 三、Web 管理界面

zeta-serve 内置一个零依赖的 Web 管理界面，模板和静态资源通过 `go:embed` 编译进二进制，无需额外部署前端服务。

### 3.1 技术选型

| 技术 | 用途 |
|------|------|
| Go `html/template` | 服务端模板渲染 |
| HTMX | 局部页面更新（无需 SPA 框架） |
| Chroma | 代码语法高亮 |
| Goldmark | Markdown 渲染 |
| 内嵌静态资源 | CSS、字体、JS 全部编译进二进制 |

### 3.2 功能页面

| 页面 | 路径 | 说明 |
|------|------|------|
| 登录 | `/login` | 用户名密码登录，JWT Cookie 会话 |
| 仓库列表 | `/repos` | 搜索浏览所有可见仓库 |
| 我的仓库 | `/my-repos` | 个人命名空间下的仓库 |
| 新建仓库 | `/repos/new` | 创建仓库（选择命名空间、可见性、默认分支） |
| 命名空间列表 | `/namespaces` | 浏览/创建分组命名空间 |
| 命名空间详情 | `/{namespace}` | 该命名空间下的所有仓库 |
| 仓库详情 | `/{namespace}/{repo}` | 仓库信息卡 + 根目录树 |
| 文件树 | `/{namespace}/{repo}/tree` | 异步加载子目录（HTMX） |
| 文件查看器 | `/{namespace}/{repo}/blob/{rev}/*` | 语法高亮、Markdown 预览、图片预览 |
| 提交历史 | `/{namespace}/{repo}/commits` | 分页提交列表 |
| 提交详情 | `/{namespace}/{repo}/commit/{hash}` | 提交信息 + 逐文件 Diff |
| 分支列表 | `/{namespace}/{repo}/branches` | 所有分支 |
| 标签列表 | `/{namespace}/{repo}/tags` | 所有标签 |
| 仓库设置 | `/{namespace}/{repo}/settings` | 成员管理、可见性、描述、默认分支 |
| 个人设置 | `/account` | 修改昵称、邮箱、密码 |
| SSH 密钥管理 | `/account/keys` | 添加/删除个人 SSH 公钥 |
| 用户管理 | `/admin/users` | 管理员：用户 CRUD、锁定/解锁、提权/降权、重置密码 |
| 用户密钥管理 | `/admin/users/{uid}/keys` | 管理员：代用户管理 SSH 公钥 |

### 3.3 访问控制

Web 界面采用基于 JWT Cookie 的会话认证：

- 登录验证用户名密码（Argon2id 哈希）
- 发放 24 小时有效的 JWT，存储在 `zeta_web_session` Cookie 中
- 每次请求通过中间件验证 JWT 并注入用户上下文
- 未认证请求重定向到登录页

管理员专属路由（`/admin/users/*`）通过独立的 `webAdminMiddleware` 守卫，仅允许 `admin=true` 的用户访问。

---

## 四、REST API

zeta-serve 提供完整的 RESTful API（`/api/v1` 前缀），支持 HTTP Basic Auth 和 Bearer JWT 两种认证方式。

### 4.1 认证方式

| 方式 | Header | 说明 |
|------|--------|------|
| Basic Auth | `Authorization: Basic <base64>` | 用户名:密码 |
| Bearer JWT | `Authorization: Bearer <jwt>` | 仓库级 JWT（含 UID + RID + 操作权限） |

### 4.2 API 端点

#### 用户管理

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/users` | admin | 分页列出用户 |
| POST | `/api/v1/users` | admin | 创建用户 |
| GET | `/api/v1/users/{uid}` | self/admin | 获取用户详情 |
| PUT | `/api/v1/users/{uid}` | self/admin | 更新用户资料 |
| DELETE | `/api/v1/users/{uid}` | admin | 删除用户（有名下仓库时仅锁定） |
| PUT | `/api/v1/users/{uid}/lock` | admin | 锁定用户 |
| PUT | `/api/v1/users/{uid}/unlock` | admin | 解锁用户 |
| GET | `/api/v1/users/me` | self | 当前用户信息 |
| PUT | `/api/v1/users/me/password` | self | 修改密码（需验证旧密码） |

#### SSH 密钥管理

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/users/{uid}/keys` | self/admin | 列出用户的公钥 |
| POST | `/api/v1/users/{uid}/keys` | self/admin | 添加公钥 |
| GET | `/api/v1/users/{uid}/keys/{kid}` | self/admin | 获取单个公钥 |
| DELETE | `/api/v1/users/{uid}/keys/{kid}` | self/admin | 删除公钥 |

#### 仓库浏览

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/repos` | auth | 分页列出可见仓库 |
| GET | `/api/v1/repos/{namespace}/{repo}` | repo-member | 仓库元数据 |
| GET | `/api/v1/repos/{namespace}/{repo}/tree` | repo-member | 目录树 (`?rev=&path=`) |
| GET | `/api/v1/repos/{namespace}/{repo}/blob/{rev}/*` | repo-member | 原始文件内容（支持 Range） |
| GET | `/api/v1/repos/{namespace}/{repo}/commits` | repo-member | 提交历史 (`?rev=&path=&page=`) |
| GET | `/api/v1/repos/{namespace}/{repo}/commits/*` | repo-member | 单个提交详情 |
| GET | `/api/v1/repos/{namespace}/{repo}/branches` | repo-member | 分支列表 (`?search=`) |
| GET | `/api/v1/repos/{namespace}/{repo}/branches/{name}` | repo-member | 单个分支 |
| GET | `/api/v1/repos/{namespace}/{repo}/tags` | repo-member | 标签列表 (`?search=`) |
| GET | `/api/v1/repos/{namespace}/{repo}/tags/{name}` | repo-member | 单个标签 |

#### 成员管理

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/repos/{namespace}/{repo}/members` | master+ | 列出仓库成员 |
| POST | `/api/v1/repos/{namespace}/{repo}/members` | owner | 添加成员 |
| PUT | `/api/v1/repos/{namespace}/{repo}/members/{uid}` | owner | 更新成员权限 |
| DELETE | `/api/v1/repos/{namespace}/{repo}/members/{uid}` | owner | 移除成员 |

#### 命名空间管理

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/namespaces` | auth | 列出命名空间 (`?type=user|group`) |
| POST | `/api/v1/namespaces` | auth | 创建分组命名空间 |

#### 管理/种子 API

以下端点用于测试和初始化，生产环境建议使用正式 API：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/user` | 创建用户（管理 API） |
| POST | `/api/v1/key` | 添加 SSH 公钥（管理 API） |
| POST | `/api/v1/repo` | 创建仓库（管理 API） |

### 4.3 API 使用示例

```shell
# 设置用户信息
export ZETA_SERVER="http://127.0.0.1:21000"

# 获取当前用户信息
curl -u admin:password ${ZETA_SERVER}/api/v1/users/me

# 创建用户
curl -u admin:password -X POST ${ZETA_SERVER}/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","name":"Alice","email":"alice@example.com","password":"secret"}'

# 创建命名空间
curl -u alice:secret -X POST ${ZETA_SERVER}/api/v1/namespaces \
  -H "Content-Type: application/json" \
  -d '{"path":"team-a","name":"Team A","description":"Team A repos"}'

# 列出仓库树
curl -u alice:secret ${ZETA_SERVER}/api/v1/repos/team-a/my-repo/tree?rev=mainline&path=src

# 添加仓库成员
curl -u alice:secret -X POST ${ZETA_SERVER}/api/v1/repos/team-a/my-repo/members \
  -H "Content-Type: application/json" \
  -d '{"uid":2,"access_level":30,"expires_at":"2027-12-31T00:00:00Z"}'
```

---

## 五、访问控制模型

### 5.1 仓库可见性

| 级别 | 值 | 说明 |
|------|----|------|
| Private | 0 | 仅仓库成员可读 |
| Internal | 10 | 内部员工可读，外包用户不可读 |
| Public | 20 | 所有登录用户可读 |
| Anonymous | 30 | 匿名可读 |

### 5.2 成员访问级别

| 级别 | 值 | 可读 | 可写 | 可管理 |
|------|----|------|------|--------|
| None | 0 | | | |
| Reporter | 20 | ✓ | | |
| Developer | 30 | ✓ | ✓ | |
| Master | 40 | ✓ | ✓ | ✓ (Sudo) |
| Owner | 50 | ✓ | ✓ | ✓ (全权限) |

### 5.3 分支保护级别

| 级别 | 值 | 说明 |
|------|----|------|
| General | 0 | 普通分支，所有人可推送 |
| Protected | 10 | 保护分支，需 Developer+ 权限 |
| Archived | 20 | 归档分支，只读 |
| Confidential | 30 | 机密分支，需更高权限 |

### 5.4 用户类型

| 类型 | 值 | 说明 |
|------|----|------|
| Normal | 0 | 普通用户 |
| Bot | 1 | 机器人账户 |
| RemoteUser | 2 | 外部用户（受限，无法访问 Internal 仓库） |

---

## 六、Z1 线协议

Z1 是 zeta 客户端与服务端之间的 HTTP 传输协议。客户端发送 `Zeta-Protocol: z1` 头进行协议识别。

### 6.1 传输端点

所有端点位于 `/{namespace}/{repo}/` 路径下：

| 方法 | 路径 | 操作 | 说明 |
|------|------|------|------|
| POST | `/authorization` | - | 获取仓库级别的签名 JWT（share 认证） |
| GET | `/reference/*` | DOWNLOAD | 获取引用列表（ls-remote） |
| POST | `/reference/*` | UPLOAD | 推送提交或批量检查大对象 |
| PUT | `/reference/*` | UPLOAD | 上传单个大对象 |
| POST | `/metadata/batch` | DOWNLOAD | 批量获取元数据 |
| GET | `/metadata/*` | DOWNLOAD | 获取提交/树元数据 |
| POST | `/metadata/*` | DOWNLOAD | 稀疏检出元数据 |
| POST | `/objects/batch` | DOWNLOAD | 批量下载对象 |
| POST | `/objects/share` | DOWNLOAD | 获取 OSS 签名 URL（直连下载） |
| GET | `/objects/{oid}` | DOWNLOAD | 下载单个对象 |

### 6.2 认证

Z1 协议使用 HTTP Basic Auth（用户名:密码）或 Bearer JWT（仓库级 JWT，含 UID、RID、操作权限）。

### 6.3 OSS 直连下载

开启 `core.accelerator = direct` 后，客户端通过 `/objects/share` 获取 OSS 预签名 URL，直接从对象存储下载 Blob，绕过 zeta-serve 转发，大幅提升吞吐：

```
客户端              zeta-serve              OSS
  │  POST /objects/share   │                  │
  │ ─────────────────────> │                  │
  │    返回签名 URL列表      │                  │
  │ <───────────────────── │                  │
  │  GET <signed-oss-url>  │                  │
  │ ─────────────────────────────────────────> │
  │       Blob 数据         │                  │
  │ <───────────────────────────────────────── │
```

---

## 七、SSH 协议

zeta-serve SSH 服务端兼容 SSH 公钥认证，用户将公钥上传到 Web 界面或通过 API 设置后，即可通过 SSH 协议访问仓库。

### 7.1 支持的命令

```shell
# 列出引用（分支/标签/引用）
zeta-serve ls-remote "group/repo" --reference mainline

# 获取元数据（提交/树）
zeta-serve metadata "group/repo" --revision ${REVISION} --depth=1
zeta-serve metadata "group/repo" --revision ${REVISION} --sparse --depth=1
zeta-serve metadata "group/repo" --batch --depth=1

# 下载对象
zeta-serve objects "group/repo" --oid=${OID}
zeta-serve objects "group/repo" --batch
zeta-serve objects "group/repo" --share

# 推送
zeta-serve push "group/repo" --reference ${REFNAME} --old-rev ${OLD} --new-rev ${NEW}
zeta-serve push "group/repo" --reference ${REFNAME} --oid ${OID} --size ${SIZE}
zeta-serve push "group/repo" --reference ${REFNAME} --batch-check
```

### 7.2 客户端配置

```shell
# 设置 zeta 用户信息
zeta config --global user.name "Your Name"
zeta config --global user.email "your@email.com"

# 设置远程地址（SSH 协议）
zeta config --global core.remote "zeta@zeta.example.com:group/repo"

# 或 HTTP 协议
zeta config --global core.remote "https://zeta.example.com/group/repo"

# 开启 OSS 直连下载（推荐）
zeta config --global core.accelerator direct
```

### 7.3 部署密钥

除了用户个人的 SSH 公钥，zeta-serve 还支持**部署密钥**（Deploy Key）——与特定仓库绑定的公钥，不关联用户身份，仅对该仓库有下载权限。适用于 CI/CD 等自动化场景。

---

## 八、配置参考

### 8.1 HTTP Server 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `listen` | string | `127.0.0.1:21000` | 监听地址 |
| `repositories` | string | - | 仓库本地缓存根目录 |
| `banner_version` | string | 服务版本 | 服务标识 |
| `x25519_key` | string | - | X25519 私钥（PEM），用于配置解密 |
| `read_timeout` | duration | `2h` | 读超时 |
| `write_timeout` | duration | `2h` | 写超时 |
| `idle_timeout` | duration | `5m` | 空闲超时 |

### 8.2 SSH Server 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `listen` | string | `127.0.0.1:22000` | 监听地址 |
| `endpoint` | string | `zeta.io` | 客户端连接用的 host（生成 remote URL） |
| `repositories` | string | - | 仓库本地缓存根目录 |
| `host_private_keys` | []string | - | SSH 主机密钥（PEM 格式） |
| `banner_version` | string | 服务版本 | 服务标识 |
| `x25519_key` | string | - | X25519 私钥（PEM），用于配置解密 |
| `max_timeout` | duration | `2h` | 最大超时 |
| `idle_timeout` | duration | `5m` | 空闲超时 |

### 8.3 数据库配置 (`[database]`)

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `name` | string | 数据库名称 |
| `user` | string | 数据库用户名 |
| `host` | string | 数据库主机 |
| `port` | int | 数据库端口 |
| `passwd` | string | 数据库密码（可加密） |
| `timeout` | duration | 连接超时（默认 30s） |

### 8.4 OSS 配置 (`[oss]`)

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `endpoint` | string | OSS 端点 |
| `shared_endpoint` | string | 直连下载端点（可选，用于 OSS 预签名） |
| `bucket` | string | Bucket 名称 |
| `access_key_id` | string | Access Key ID（可加密） |
| `access_key_secret` | string | Access Key Secret（可加密） |
| `product` | string | 产品标识 |
| `region` | string | 区域 |

### 8.5 缓存配置 (`[cache]`)

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `num_counters` | int64 | `1000000000` | Ristretto 缓存计数器 |
| `max_cost` | int64 | `20` | 最大缓存代价 |
| `buffer_items` | int64 | `64` | 缓冲项 |

### 8.6 管理员配置 (`[admin]`)

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `username` | string | 管理员用户名 |
| `password` | string | 管理员密码（可加密） |
| `email` | string | 管理员邮箱（可选） |

> 该配置仅用于首次启动时的管理员种子创建，不可重复创建已有账号，不会覆盖已存在的同名用户。

---

## 九、对象存储布局

Blob 内容通过 ODB 层写入 OSS，存储路径按仓库 ID 分片以避免热分区：

```
zeta/{rid%1000}/{rid}/{hash[0:2]}/{hash[2:4]}/{hash}
```

例如仓库 ID 为 42、对象哈希为 `abc123...` 的路径为：

```
zeta/042/42/ab/c1/abc123...
```

### 对象存储路径设计要点

- 前两位哈希分片 (`ab/c1/`) 避免单目录下对象过多
- `rid%1000` 前缀分散写入压力
- 支持断点上传：相同 OID 重复写入时通过 `Stat` 检查跳过
- 支持哈希校验：上传时并发计算对象哈希，写入后验证一致性

---

## 十、数据库 Schema

| 表 | 说明 |
|------|------|
| `users` | 用户账户（用户名、密码哈希、管理员标志、签名 Token） |
| `namespaces` | 命名空间（用户类型 / 分组类型） |
| `repositories` | 仓库元数据（名称、路径、可见性、默认分支、哈希/压缩算法） |
| `branches` | 分支引用（哈希、保护级别） |
| `tags` | 标签引用（哈希、标题、描述） |
| `refs` | 普通引用 |
| `objects` | 扩展元数据对象（编码对象二进制） |
| `commits` | 提交记录（作者、提交者、编码数据） |
| `trees` | 树对象（编码数据） |
| `members` | 仓库成员（访问级别、过期时间） |
| `ssh_keys` | SSH 公钥（用户公钥 / 部署公钥） |
| `deploy_keys_repositories` | 部署密钥与仓库的绑定关系 |
| `emails` | 用户邮箱映射 |

Schema 完整定义见 `pkg/serve/database/zeta.sql`。

---

## 十一、安全机制

### 11.1 密码安全

- 用户密码使用 **Argon2id** 算法加密存储
- Web 登录、API Basic Auth 均通过 Argon2id 验证
- 服务端响应中永不返回密码字段（`User.Guard()` 清除内存中的密码）

### 11.2 配置加密

- X25519 ECIES（AES-256-GCM + HKDF-SHA256）方案加密配置中的敏感字段
- 加密密文以 `ENC@` 前缀标识，非加密文本原样透传
- 通过 `zeta-serve encrypt` 命令生成密文

### 11.3 JWT 认证

- **仓库级 JWT（Bearer）**：包含 UID、RID、操作权限（DOWNLOAD/UPLOAD/PSEUDO），用于 Z1 协议
- **Web 会话 JWT**：包含 UID，24h 有效期，存储在 Cookie 中
- JWT 使用用户的 `SignatureToken`（随机生成）作为 HS256 签名密钥

### 11.4 访问控制

- 仓库操作前进行 `RepoAccessLevel` 检查
- 管理员拥有所有仓库的全部权限
- 分支保护级别限制推送权限
- 用户锁定后禁止所有操作

---

## 十二、典型部署场景

### 场景一：小型团队私有化部署

```
1台服务器 → zeta-serve (httpd + sshd) + MySQL + MinIO
```

适合 5-20 人团队，单机部署即可：

```shell
# 启动 httpd 和 sshd
zeta-serve httpd -c config.toml &
zeta-serve sshd -c config.toml &

# 团队成员通过 Web 界面注册/由管理员创建
# 添加 SSH 公钥后即可通过 SSH 协议 clone/push
zeta checkout zeta@zeta.example.com:team-a/my-repo
```

### 场景二：企业级私有化部署

```
负载均衡 → zeta-serve httpd × N  (无状态，可水平扩展)
         → zeta-serve sshd  × N
共享后端 → OceanBase 集群 + OSS（阿里云 / 自建）
```

无状态服务节点可任意扩缩容。元数据集中在 OceanBase，Blob 内容集中在 OSS。

### 场景三：AI 模型版本管理

```shell
# 管理员创建仓库
curl -u admin:passwd -X POST ${SERVER}/api/v1/repos \
  -d '{"path":"ai-models","description":"LLM checkpoints","default_branch":"mainline","namespace_path":"ml-team","visible_level":0}'

# 研究员检出仓库
zeta checkout https://zeta.example.com/ml-team/ai-models

# 提交大模型 checkpoint（数十 GB）
zeta add model-v2.ckpt
zeta commit -m "add model v2 checkpoint"
zeta push

# 增量推送：Fragments + CDC 自动分片，仅传输变化的分片
```

### 场景四：CI/CD 自动化集成

```shell
# 使用部署密钥或 Bot 账户
export ZETA_REMOTE="zeta@zeta.example.com:ci/artifacts"

# 在流水线中拉取和推送
zeta checkout ${ZETA_REMOTE} --one artifacts
zeta add build-output/
zeta commit -m "CI: build #${BUILD_ID}"
zeta push
```

---

## 十三、反向代理与 TLS 终端

生产环境推荐在 zeta-serve 前部署反向代理（如 Nginx、Caddy、Envoy），由代理层统一处理 TLS 证书、域名路由、端口暴露和安全加固。zeta-serve 本身不内建 HTTPS，仅需监听内网端口，对外流量交给代理层。

```
用户 / zeta client
     │
     ▼ HTTPS / SSH
┌──────────────────────────────────────────────────┐
│              Nginx / Caddy（反向代理）             │
│  -- TLS 证书终止 (Let's Encrypt / 企业 CA)        │
│  -- HTTP 443  →  upstream 127.0.0.1:21000        │
│  -- SSH   22   →  upstream 127.0.0.1:22000        │
└──────────────┬───────────────────────────────────┘
               │
               ▼
        zeta-serve (httpd + sshd)
```

### 13.1 为什么需要反向代理

| 能力 | 说明 |
|------|------|
| **TLS / HTTPS** | zeta-serve 不内建 TLS，由代理统一终止 HTTPS，管理证书续期 |
| **标准端口映射** | 对外暴露 443/22，内部 21000/22000 不直接触达 |
| **负载均衡** | 多实例 httpd 无状态可水平扩展，代理做健康检查和轮询 |
| **安全加固** | 限流、IP 白名单、WAF、SSH 端口跳转等 |
| **大文件传输** | 调整 `proxy_read_timeout`、`proxy_send_timeout`、关闭 `proxy_buffering` 以支持流式传输 |
| **Gzip 压缩** | 代理层对 Web 页面和 API JSON 做响应压缩 |

### 13.2 Nginx 配置示例

#### HTTP 反向代理（HTTPS 终端）

```nginx
upstream zeta_http {
    server 127.0.0.1:21000;
    # 多实例时增加更多 server，加 max_fails/fail_timeout
    # server 10.0.0.2:21000 max_fails=3 fail_timeout=10s;
}

server {
    listen 443 ssl http2;
    server_name zeta.example.com;

    # TLS 证书
    ssl_certificate     /etc/nginx/ssl/zeta.example.com.crt;
    ssl_certificate_key /etc/nginx/ssl/zeta.example.com.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # 客户端上传体积（AI 模型等大文件场景需调大）
    client_max_body_size 0;  # 0 = 不限制

    # Z1 协议传输大对象，需关闭缓冲以支持流式
    proxy_buffering off;
    proxy_request_buffering off;

    # 超时设置——大文件 push 时耗时较长
    proxy_read_timeout  2h;
    proxy_send_timeout  2h;

    # 透传客户端真实 IP
    proxy_set_header Host              $host;
    proxy_set_header X-Real-IP         $remote_addr;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    location / {
        proxy_pass http://zeta_http;
    }
}

# HTTP → HTTPS 重定向
server {
    listen 80;
    server_name zeta.example.com;
    return 301 https://$host$request_uri;
}
```

#### SSH 反向代理（TCP 四层转发）

zeta-serve 的 SSH 服务无法直接走 HTTP 代理，需使用 Nginx 的 `stream` 模块做 TCP 转发：

```nginx
# nginx.conf 顶层（stream 模块与 http 同级）
stream {
    upstream zeta_ssh {
        server 127.0.0.1:22000;
    }

    server {
        listen 22;  # 标准 SSH 端口
        proxy_pass zeta_ssh;
        proxy_timeout 2h;
        # 可选：SSH 连接超时
        proxy_connect_timeout 10s;
    }
}
```

> 若 Nginx 已占用 22 端口（例如同时代理 git-ssh），可用非标准端口（如 2222），客户端通过 `zeta@zeta.example.com -p 2222` 连接，或换用独立的入口域名。

### 13.3 TLS 证书管理

#### Let's Encrypt + Certbot 自动续期

```shell
# 安装 certbot
apt install certbot python3-certbot-nginx

# 签发证书（Nginx 插件自动配置）
certbot --nginx -d zeta.example.com

# 自动续期（cron）
echo "0 3 * * * certbot renew --quiet --post-hook 'nginx -s reload'" | crontab -
```

#### Caddy 自动 HTTPS

Caddy 内建 Let's Encrypt 自动签发与续期，配置更简洁：

```
zeta.example.com {
    reverse_proxy 127.0.0.1:21000 {
        flush_interval -1  # 流式传输，不缓冲
        transport http {
            read_timeout 2h
            write_timeout 2h
        }
    }
}
```

#### 企业自签名 CA

私有环境可使用内部 CA 签发证书，客户端信任根证书即可：

```shell
# 客户端跳过 SSL 校验（仅内网环境）
zeta config --global http.sslVerify false

# 或将企业 CA 证书加入系统信任链
cp my-company-ca.crt /usr/local/share/ca-certificates/
update-ca-certificates
```

### 13.4 完整部署拓扑参考

```
                    互联网 / 内网
                        │
              ┌─────────┴─────────┐
              │  DNS zeta.example.com│
              └─────────┬─────────┘
                        │
         ┌──────────────┴──────────────┐
         │         Nginx / Caddy        │
         │  443 (HTTPS) → :21000      │
         │  22  (SSH)   → :22000      │
         └──────┬────────────┬────────┘
                │            │
     ┌──────────┘            └──────────┐
     │                                  │
     ▼ (×n 实例)           ▼ (×n 实例)
 zeta-serve httpd          zeta-serve sshd
     │           \    /           │
     │            \  /            │
     ▼             ▼             ▼
  ┌──────────────────────────────┐
  │     OceanBase / MySQL        │
  │     () 纳入分布式数据库)         │
  └──────────────────────────────┘
               +
  ┌──────────────────────────────┐
  │     OSS / S3 / MinIO         │
  │     ( Blob 内容存储)           │
  └──────────────────────────────┘
```

### 13.5 关键注意事项

| 事项 | 说明 |
|------|------|
| `proxy_buffering off` | Z1 协议传输大对象需要流式透传，开启缓冲会导致内存占用峰值和超时 |
| `client_max_body_size 0` | Z1 协议 push 时通过 PUT 上传大对象 body，需解除代理层体积限制 |
| `proxy_read_timeout 2h` | 与 zeta-serve 的 `read_timeout` 对齐，避免代理层提前断开 |
| X-Forwarded-Proto | 使用 HTTPS 时透传该头，保证 Web 界面生成的回调 URL 协议正确 |
| SSH 端口冲突 | 若服务器已有 openssh，zeta-serve sshd 使用非标准端口或通过 stream 转发 |

---

## 十四、运维操作

### 14.1 日志

服务日志通过 `logrus` 输出到标准输出/标准错误，包含请求方法、路径、状态码、收发字节数、耗时等信息：

```
[10.0.0.1] GET /repos/new status: 200 received: 0 written: 4096 spent: 1.2ms
[10.0.0.1] POST /group/repo/reference/mainline status: 200 received: 4194304 written: 0 spent: 2.3s
```

### 14.2 优雅关闭

服务监听 SIGINT/SIGTERM 信号，收到信号后等待进行中的请求完成后退出。

### 14.3 命令行子命令

| 子命令 | 说明 |
|--------|------|
| `zeta-serve httpd` | 启动 HTTP 服务 |
| `zeta-serve sshd` | 启动 SSH 服务 |
| `zeta-serve keygen` | 生成密钥 (RSA/ED25519/ECDSA/X25519) |
| `zeta-serve encrypt` | 加密配置值 |

全局标志：

| 标志 | 说明 |
|------|------|
| `-V, --verbose` | 调试模式 |
| `-E, --expand-env` | 展开配置中的环境变量 |
| `-v, --version` | 显示版本 |
| `-c, --config` | 指定配置文件路径 |

---

## 十五、客户端连接

### 15.1 通过 HTTP

```shell
# 检出
zeta checkout https://zeta.example.com/group/repo

# 推送
zeta push

# 拉取
zeta pull
```

### 15.2 通过 SSH

```shell
# 检出
zeta checkout zeta@zeta.example.com:group/repo

# 推送
zeta push
```

### 15.3 配置下载加速

```shell
# 开启 OSS 直连下载（推荐，大幅提升大文件下载速度）
zeta config --global core.accelerator direct

# 多线程下载（aria2）
zeta config --global core.accelerator aria2

# P2P 加速（Dragonfly）
zeta config --global core.accelerator dragonfly
```

---

## 十六、数据分离架构

zeta-serve 的核心设计是**元数据与内容数据分离**：

```
                     元数据 (DB)                    内容数据 (OSS)
 ┌──────────────────────────────────┐  ┌──────────────────────────────┐
 │ commits       - 编码的提交对象     │  │ blobs   - 压缩的文件内容      │
 │ trees         - 编码的目录树对象   │  │ fragments - 大文件分片       │
 │ branches      - 分支引用           │  └──────────────────────────────┘
 │ tags          - 标签引用           │
 │ objects       - 扩展元数据对象     │
 │ users/members - 权限管理          │
 └──────────────────────────────────┘
```

优势：
- 元数据小而紧凑，适合索引和快速查询
- Blob 内容存储在分布式对象存储，可弹性扩展
- 检出按需拉取（metadata + 所需 blob），而非全量克隆
- 大文件通过 Fragment 自动分片 + CDC 增量传输

---

## 十七、相关文档

| 文档 | 说明 |
|------|------|
| [README.md](README.md) | 文档中心索引 |
| [design.md](design.md) | HugeSCM 核心设计哲学 |
| [protocol.md](protocol.md) | 传输协议规范 |
| [config.md](config.md) | 客户端配置文件说明 |
| [object-format.md](object-format.md) | 对象格式详解 |
| [pack-format.md](pack-format.md) | 打包格式 |
| [version-negotiation.md](version-negotiation.md) | 版本协商机制 |
| [sparse-checkout.md](sparse-checkout.md) | 稀疏检出 |
| [cdc.md](cdc.md) | CDC 分片原理 |
