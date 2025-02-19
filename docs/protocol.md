# HugeSCM 传输协议规范

## 一、协议约定
早期在我们设计 HugeSCM 传输协议时，我们对 HugeSCM 的设计存在认识不足，没有充分考虑到实际需求，此外，在 HugeSCM 的推广过程，我们也发现 HugeSCM 需要引入一些设计扩展，以支持 HugeSCM 的功能扩展，因此，在我们专门引入了 HugeSCM 传输协议规范，制定相关约束。

### 1.1 版本协商
在采用 HugeSCM 传输协议下载/上传数据时，应正确设置传输协议版本，服务端根据传输协议版本选择合适的实现，其中。

本规范的传输协议字符串为：`Z1`

HTTP 请求需设置请求头 `Zeta-Protocol: Z1`。

SSH 请求需设置环境变量：`ZETA_PROTOCOL=Z1`

后续如果有新的协议引入，则使用字符串：`Z2 Z3 ... ZN`。

### 1.2 授权
#### 1.2.1 HTTP 验证
HugeSCM 的传输协议支持用户名和密码（Token）的验证方式，支持的授权方式有 `Basic`以及 `Bearer`。

对于 Basic 授权，我们支持：`邮箱+密码`，`域账号+密码`，`允许的用户名+token`。

为了提高服务端的安全性，我们还引入了签名验证机制，在本协议中，我们使用 Bearer 验证机制，即使用 JWT 签名。

用户在请求 `{namespace}/{repo}/authorization` 接口时，我们先验证用户权限，如果权限 OK，我们将使用特定的算法，生成一个 Bearer Token，客户端后续使用该 token 操作即可。

请求体：

```json
{
    "operation": "download",
    "version": "0.12.3"
}
```

这里的 `operation`有效值是 `download`和 `upload`，客户端如果想要检查是否有写入权限，则可以指定 `upload`，否则指定 `download`即可，因为我们在后续的协议中会再度检查用户的权限。而 `version`用于告诉服务端客户端的版本。

返回：

```json
{
    "header": {
        "authorization": "Bearer *****"
    },
    "notice": "可选",
    "expires_at": "2023-12-20T17:54:49.244244+08:00"
}
```

客户端可以检测 `expires_at`确认 token 是否过期，可以使用我们提供的 `authorization`设置到 HTTP 请求头，当然用户可以不使用该机制，使用标准的 Basic 验证也是支持的。该接口返回的 `notice`，客户端可以将该通知/提示输出到终端。

#### 1.2.2 SSH 验证
SSH 传输协议可以使用 SSH 公钥进行验证，与 SSH 相同，这里不做赘述。

## 二、下载数据协议集
本章内容主要是介绍如何实现下载数据的传输协议集，便于用户从远程存储获取所需的数据，从而在本地创建存储库的快照，本协议集即需要支持稀疏的，浅表的存储库数据获取，也需要具备完全的存储库数据下载能力，在 HugeSCM 中，我们的遵循的原则都是单分支/单标签的数据下载，而不像 Git 那样，下载所有的存储库数据，因为在举行存储库中，无论如何，将存储库的数据完全下载到本地都是不经济的，没有必要的。

| 名称 | 匹配 | 备注 |
| --- | --- | --- |
| 引用发现 | `GET /{namespace}/{repo}/reference/{refname}` | `Accept: application/vnd.zeta+json` |
| 元数据 | `GET /{namespace}/{repo}/metadata/{revision:.*}`<br/>`POST /{namespace}/{repo}/metadata/{revision:.*}`<br/>`POST /{namespace}/{repo}/metadata/batch` | 在这里 `revision`只能是 `commit`或者 `tag`对象，不能是 `tree`或者其他。<br/>可设置 `deepen-from`和 `deepen`，分别表示从那个 commit 开始或者回溯深度，deepen-from 默认没有设置，而 deepen 如果没有设置就使用默认值 1.<br/>其中批量元数据下载不支持 `deepen-from`和 `deepen`。 |
| blob | `POST /{namespace}/{repo}/objects/batch`<br/>`POST /{namespace}/{repo}/objects/shared`<br/>`GET /{namespace}/{repo}/objects/{oid}` | 在这里我们需要支持批量下载小文件，也需要支持下载大文件，此外还需要支持签名下载对象，支持签名下载的好处是，我们可以减少网络带宽的消耗。 |


### 2.1 引用发现协议
在 HugeSCM 中，我们目前设计了分支发现协议和标签发现协议，以支持用户获得存储库的分支/标签信息，并且在返回中包含存储库的哈希算法，默认分支，压缩算法，以及 capabilities 等信息，客户端可以根据 capabilities 信息感知服务端的能力。

由于 HugeSCM 的特殊设计，我们并不需要像 Git 那样将所有的引用数据都传输给客户端，因此我们完全可以将引用发现协议的返回数据设置`Content-Type: application/vnd.zeta+json`，以降低解析数据的难度。

假如 zeta 存储库的 remote 为：`https://zeta.io/group/mono-zeta` ，那么我们可以通过：

```bash
# Get ref information
GET "https://zeta.io/group/mono-zeta/reference/${REFNAME}"
# SSH command
zeta-serve ls-remote "group/mono-zeta" --reference "${REFNAME}"
```

计算分支/标签的名称：

+ 分支：`refs/heads/`+`branch`
+ 标签：`refs/tags/`+`tag` 
+ 其他：待补充

客户端需要设置：`Accept: application/vnd.zeta+json`

引用的返回格式如下：

```json
{
  "remote": "https://zeta.io/zeta/zeta-mono",
  "name": "refs/tags/v1.0.0",
  "hash": "9b724e5d1e1434ea916feaa3f1c2d3e467058c6bdab1b34fe9752550451a7039",
  "peeled": "6d2eb25e45c4f5135da48e786cbb4c8af06a6009ecd679e0547c06a640bbc310",
  "head": "refs/heads/mainline",
  "version": 1,
  "agent": "Zeta-1.0",
  "hash-algo": "BLAKE3",
  "compression-algo": "zstd",
  "capabilities": []
}
```

+ remote 即远程存储库地址，保留。
+ name 即当前的引用的名称。
+ hash 即 v1.0.0 分支的最新提交。
+ peeled 是可选的，如果一个引用是 tag，并且是从 git 迁移过来的，可能是 tag 对象，服务端应返回去皮 tag，如果不是则省略。
+ head，通常是默认分支。
+ version 即 zeta 协议版本。
+ agent zeta 服务端版本。
+ hash-algo 则是哈希算法。
+ compression-algo 压缩算法。
+ capabilities 预留能力。

错误返回格式为：

+ code 错误码
+ message 错误信息

比如引用不存在，则返回 404。

```json
{
  "code":404,
  "message":"repo linkcs not exist"
}
```

### 2.2 元数据传输协议
HugeSCM 元数据传输协议，支持的 Query 分别有：

+ `deepen-from`值为 commit 的哈希，从某个 commit 开始到指定 commit 之前所有的提交和 tree，fragments 等元数据集合。
+ `deepen`值类型为正整数，即获取 deepen 个提交的元数据集合，如果设置了 `deepen-from`则忽略 `deepen`，未设置 `deepen`时，我们默认会获取 commit 一个提交包含的元数据。
+ `depth`目录层级深度，未设置则获得所有的 tree。

#### 2.2.1 编码格式
在 HugeSCM 中，方案规定，metadata 数据格式为：

1. 4 字节 MAGIC，目前的定义为 `'Z','M','\x00','\x01'`
2. 4 字节 Version，当前值为 1。
3. 16 字节 Reserved 保留字段，全部填充为 `'\0'`。
4. 4 字节的 object_length，这个即 `metadata_entry`的数据总长度。
5. `$object_length`字节的 `metadata_entry`包括 64 字节的哈希和二进制内容。
6. `metadata_entry`的数量是可变的，只有当接收到的 object_end 值为 0 时表示元数据传输结束。
7. 16 字节的 CRC64 (ISO) 校验合。即整个传输流的 CRC64，不包含 crc64_checksum 本身。

```cpp
struct metadata_entry {
  std::byte hash[64]; // object hash
  std::byte *content; // variable content
};

struct metadata {
  std::byte magic[4];          // 'Z','M','\x00','\x01'
  std::uint32_t version;       // VERSION default =1
  std::byte reserved[16];      // reserved: full zero
  std::uint32_t object_length; // object length - 64 == object content length
  metadata_entry entry;        // object hash and content.
  /* ... */
  std::uint32_t object_end;     // ==> 0000
  std::byte crc64_checksum[16]; // 16 byte CRC64 (ISO) checksum
};

```

无论是 Commit/Tree 还是稀疏 Commit 协议的返回都应该是符合元数据二进制格式。

客户端需要设置正确的 `Accept`：

+ `Accept: application/x-zeta-metadata` 传输流不压缩。
+ `Accept:  application/x-zeta-compress-metadata`，传输流使用 ZSTD 压缩。

SSH 协议可以添加参数 `--zstd` 开启元数据压缩。

#### 2.2.2 基本元数据下载
在 HugeSCM 系统中，只需要获得最新的 `revision`及其 tree 就行了，这里 `revision`可以是 `commit`也可以是 `tag`，如果是 `tag`对象需进一步解析到 `commit`为止。

```bash
# Get commit metadata
GET "https://zeta.io/group/mono-zeta/metadata/${REVISION}"
# SSH
zeta-serve metadata "group/mono-zeta" --revision "${REVISION}" --depth=1 --deepen-from=${from}
```

请求格式

| **参数** | **类型** | **描述** |
| --- | --- | --- |
| revision | String | 提交 ID 或 tag 对象 ID |
| depth | Integer | 可选，如果没有设置，服务端将遍历该提交所有的 tree，否则，按照 depth 指定遍历指定深度的 tree。 |
| deepen-from | Hash | 可选，将从 `deepen-from`开始的 commit 到 指定的 commit 之间所有的 commit 也返回给客户端，一旦设置了 `deepen-from`，服务端将检查 deepen- from 是否是所需 commit 的祖先，不是祖先则返回 419。 |
| have | Hash | 该值标记本地存在的 commit，在 Fetch 阶段，服务端会根据 deepen-from 以及 have 确认本地存储库已经存在哪些 commit，并轻点出所需的对象。 |
| deepen | Integer | 值类型为正整数，即获取 deepen 个提交的元数据集合，如果设置了 `deepen-from`则忽略 `deepen`，未设置 `deepen`时，我们默认会获取 commit 一个提交包含的元数据。 |


如果查询是添加了 `depth=N`，我们将限制查询 tree 的深度，`0`表示不返回任何 `tree`，默认（即 depth 参数不存在时）返回所有该 revision `root-tree`的所有 `sub-tree`。

#### 2.2.3 稀疏元数据下载
在 HugeSCM 中，我们支持稀疏元数据下载，其请求如下：

```bash
# Get commit metadata
POST "https://zeta.io/group/mono-zeta/metadata/${REVISION}"
# SSH
zeta-serve metadata "group/mono-zeta" --revision "${REVISION}" --sparse --depth=1 --deepen-from=${from}
```

客户端将请求的目录发送给服务端，服务端据此返回相应的稀疏元数据，请求格式如下：

```bash
cat <
src/link LF
src/zeta LF
LF
>
```

内容返回细节与基本元数据传输相同。

#### 2.2.4 批量元数据下载
在 HugeSCM 中，我们支持批量元数据下载，其请求如下：

```bash
# Get commit metadata
POST "https://zeta.io/group/mono-zeta/metadata/batch"
# SSH
zeta-serve metadata "group/mono-zeta" --batch --depth=1
```

客户端将请求的目录发送给服务端，服务端据此返回相应的稀疏元数据，请求格式如下：

```bash
cat <
oid LF
oid LF
LF
>
```

内容返回细节与基本元数据传输相同。

这里对不同类型的对象的返回如下：

+ tree 返回指定深度的 sub tree。
+ commit 返回根 tree 和指定深度的 sub tree。
+ fragments 返回自身。
+ tag 返回自身及其 commit 和 tree ，指定深度的 sub tree。

这里需要注意，通常情况下标准客户端可能不需要实现批量元数据下载，基本元数据下载和稀疏元数据下载已经能满足现有的需求，而批量元数据下载可以适用于 FUSE 等场景，而元数据并不像 blob 那样占据大量空间，绝大多数时候都可以完全下载到本地。

### 2.3 文件数据传输协议
本节主要描述如何实现 Blob 的下载，包含批量下载（小 blob），签名分享下载（大 blob），以及单一 blob 下载（无论大小）。

#### 2.3.1 单个下载
在 HugeSCM 中，最简单的 blob 获取方式是单个 blob 下载，请求格式如下：

```bash
# HTTP
GET "https://zeta.io/group/mono-zeta/objects/${OID}"
# SSH
zeta-serve objects group/mono-zeta --oid "${OID}" --offset=0
```

此外，客户端需要设置：`Accept: application/x-zeta-blob`。

该接口需要支持断点续传功能，即客户端在下载数据中断后，可以请求从指定位置开始下载，对于体积较大的 blob，很容易出现因网络的原因超时中断，因此，服务端需具备该能力，客户端也需要支持断点续传。

本接口返回体系 blob 的二进制内容，服务端需要在 Header 中设置 `X-Zeta-Compressed-Size: $compressed_size`，或者正确设置 `Content-Length`，保证断点续传功能正常运行。

在 SSH 协议中，单个对象下载与 HTTP 的返回是不同，HTTP 返回的是 BLOB 对象的内容（端点下载的内容），而 SSH 协议需要保留一定长度的元数据：

1. 4 字节的 MAGIC，目前是 `'Z', 'B', '\x00', '\x02'`。
2. 4 字节 Version，当前值为 `1`。
3. 8 字节当前 BLOB 传输长度。
4. 8 字节当前 BLOB 压缩长度。

#### 2.3.2 批量下载
批量下载是返回用户的请求所需的 blob，请求格式如下：

```bash
POST "https://zeta.io/group/mono-zeta/objects/batch"
# SSH
zeta-serve objects group/mono-zeta --batch
# -----
cat <
oid LF
oid LF
...
oid LF
LF
>
```

连续两个换行符代表（`LF`）传输结束。

此外，客户端需要设置：`Accept: application/x-zeta-blobs`

批量 blob 下载二进制格式如下：

1. 4 字节的 MAGIC，目前是 `'Z', 'B', '\x00', '\x02'`。
2. 4 字节 Version，当前值为 `1`。
3. 16 字节 Reserved 保留字段，全部填充为 `'\0'`。
4. 4 字节的 entry_length，这个即`blob_entry`的数据总长度。
5. `$entry_length`字节的 `blob_entry`包括 64 字节的哈希和二进制内容。
6. `blob_entry`的数量是可变的，只有当接收到的 blob_end 值为 0 时表示元数据传输结束。
7. 16 字节的 CRC64 (ISO) 校验合。即整个传输流的 CRC64，不包含 crc64_checksum 本身。

结构体定义：

```cpp
struct blob_entry {
  std::byte hash[64]; // object hash
  std::byte *content; // variable content
};

struct batch_blob_stream {
  std::byte magic[4];         // 'Z','B','\x00','\x02'
  std::uint32_t version;      // VERSION default =1
  std::byte reserved[16];     // reserved: full zero
  std::uint32_t entry_length; // blob entry length - 64 == blob content size
  blob_entry entry;           // blob hash and content
  /* ... */
  std::uint32_t blob_end;       // ==>0000
  std::byte crc64_checksum[16]; // 16 byte CRC64 (ISO) checksum
};

```

**注意事项**：批量 blob 下载不支持传输大于 4G 的文件，因为这会降低用户体验。对于这些文件，客户端应当使用签名 URL 下载或者使用单一 blob 下载以加速下载，提高下载的稳定性。

#### 2.3.3 签名分享下载
在 HugeSCM 中，我们引入了类似 OSS 的分享签名 URL 下载特性，客户端可以将签名 URL 交由各种 P2P 客户端，比如 Dragonfly，Aria2 下载，该机制的引进能很好的解决下载加速的问题，特别是对 AI/游戏研发这种包含很多大文件，静态资源的场景，非常有裨益。

签名分享下载请求格式如下：

```bash
# HTTP
POST "https://zeta.io/group/mono-zeta/objects/share"
# SSH
zeta-serve objects group/mono-zeta --share
```

请求体的格式为 `application/vnd.zeta+json`，客户端请求时需要设置的头有 `Accept: application/vnd.zeta+json`。

```bash
{
  "objects":[
    {
      "oid":"1c3e65a02d6d6b47355ef52fd4db4f35b055dcd0bd73f27512bf05b874399378",
      "path":"os-images/AlmaLinux-8-latest-aarch64-boot.iso"
    }
  ]
}
```

以 Golang 为例定义如下：

```go
type WantObject struct {
    OID  string `json:"oid"`
}

type BatchSharedsRequest struct {
    Objects []*WantObject `json:"objects"`
}
```

该接口的返回体格式如下：

```json
{
  "objects": [
    {
      "oid": "1c3e65a02d6d6b47355ef52fd4db4f35b055dcd0bd73f27512bf05b874399378",
      "compressed_size": 857622544,
      "href": "http://zeta.oss-cn-hangzhou.aliyuncs.com/123123/1c/1c3e65a02d6d6b47355ef52fd4db4f35b055dcd0bd73f27512bf05b874399378****",
      "expires_at": "2023-11-22T22:23:33.891096+08:00"
    }
  ]
}
```

以 Golang 为例，定义如下：

```go
type Representation struct {
    OID            string            `json:"oid"`
    CompressedSize int64             `json:"compressed_size"`
    Href           string            `json:"href"`
    Header         map[string]string `json:"header,omitempty"`
    ExpiresAt      time.Time         `json:"expires_at,omitempty"`
}

type BatchSharedsResponse struct {
    Objects []*Representation `json:"objects"`
}

```

这里分别指出相应字段的含义：

+ oid - 请求对象的哈希值。
+ compressed_size - 请求 blob 的存储大小，不是 blob 对应文件的原始大小。
+ href - 请求的 URL，与 Git LFS 协议类似，客户端可以使用 href 作为下载的 URL。
+ header - 请求的 Header，与 Git LFS 协议类似，客户端需要设置 header，当然，现在默认为空。
+ expires_at - 签名 URL 过期时间，客户端在签名 URL 过期后需要重新请求新的签名 URL。

## 三、上传数据协议集
在这一章中，我们制定了上传数据的协议集，用来实现从本地将提交，修改推送到远程存储库，在维护 Git 代码托管平台的过程中，我们吸取了 git 的教训，将大文件与小文件，元数据分离开来，从而提高整个传输的稳定性，健壮性，再加上 HugeSCM 特有的分片特性，能够极大的提高整个平台的稳定性，降低网络抖动导致的推送中断重试现象。

### 3.1 文件上传检查
我们引入了文件上传检查，这个协议与 Git LFS batch API 类似，但也有一定的区别，我们没有将 download/upload 两个操作混合到一个 API，而是分离的，这样对权限校验有帮助。

请求格式如下：

```bash
# HTTP
POST https://zeta.io/group/mono-zeta/reference/{refname}/objects/batch
# SSH
zeta-serve push "group/mono-zeta" --reference "$REFNAME" --batch-check
```

请求体格式如下：

```json
{
  "objects": [
    {
      "oid": "7b5da36a30c19384275d7bf409b46a527579ecde94fdbd0175dab6f53749d280",
      "compressed_size": 111225555
    },
    {
      "oid": "17201adab16049cddd2b3d1993031091b9cdf0689f7504ed90ca0d6f5dd347bd",
      "compressed_size": 1073741840
    }
  ]
}
```

返回体格式如下：

```json
{
    "objects": [
        {
            "oid": "7b5da36a30c19384275d7bf409b46a527579ecde94fdbd0175dab6f53749d280",
            "compressed_size": 111225555,
            "action": "upload"
        },
        {
            "oid": "17201adab16049cddd2b3d1993031091b9cdf0689f7504ed90ca0d6f5dd347bd",
            "compressed_size": 1073741840,
            "action": "download"
        }
    ]
}
```

对于存在的对象，设置其 `action`为 `download`，对于不存在的对象，设置其 `action`为 `upload`，客户端根据 `action`选择上传还是跳过该 blob。

### 3.2 单一文件上传
在 HugeSCM 中，体积比较大的文件应当使用单一文件上传，建议是体积大于 20M，超过 100 M 应当使用单一文件上传，而不是将这些文件编码到推送协议一同上传。对于单一文件上传，其格式比较简单：

```bash
# HTTP
PUT https://zeta.io/group/mono-zeta/reference/{refname}/objects/{oid}
# SSH
zeta-serve push "group/mono-zeta" --reference "$REFNAME" --oid "$OID" --size "${SIZE}"
```

客户端在请求的时候，应当将 blob 的实际大小值设置到 HTTP 头 `X-Zeta-Compressed-Size`（10进制），服务端据此能绕过 OSS 大小限制（如阿里云 5GB 限制），SSH 协议请使用 `--size=N`告知服务端。

服务端选择直连上传大文件到 OSS，不过应当注意，服务端需要检测传输的 blob oid 是否与输入的 oid 相同，不同则返回错误。

此外，服务端应当检测用户是否有权限修改当前分支。

### 3.3 推送协议
在 HugeSCM 中，客户端可以使用推送协议，将本地的修改同步到远程服务器，并更新引用。请求格式如下：

```bash
# HTTP
POST "https://zeta.io/group/mono-zeta/reference/{refname}"
# SSH
zeta-serve push "group/mono-zeta" --reference "$REFNAME"
```

客户端需要设置（HTTP）：`Accept: application/x-zeta-report-result`。

此外还需要设置额外的头：

| HTTP Header | SSH 参数/环境变量 | 备注 |
| :---: | :---: | --- |
| `X-Zeta-Command-OldRev` | `--newrev` | 64 字节待更新的分支旧的哈希值，不存在使用**缺省 OID **代替。 |
| `X-Zeta-Command-NewRev` | `--oldrev` | 64 字节待更新分支新的哈希值，删除分支可以使用**缺省 OID **代替 |
| `X-Zeta-Objects-Stats` | `ZETA_OBJECTS_STATS` | 记录对象数量，服务端可以据此进行特别的优化，客户端<br/>格式为：`m-11;b-12` |

注意缺省 OID 为：`0000000000000000000000000000000000000000000000000000000000000000`

请求体的二进制格式如下：

1. 4 字节魔数，为 `'Z', 'P', '\0', '\1'`。
2. 4 字节 Version，当前为 1。
3. 16 字节保留字段，用 `'\0'`填充。
4. 8 字节条目长度（包括哈希长度），长度大于 0，则为 blob，小于 0 则为 metadata（commit/tree），等于 0 表示条目终止。（对于 metadata，其长度写入时，如 X 写入 `uint64(-(X+64))`，读取时，使用 `int64(X)`判断其大小即可。）
5. 16 字节 CRC64（ISO）校验和，不包含其本身。



以下是二进制格式定义：

```cpp
struct object_entry {
  std::byte hash[64]; // object hash
  std::byte *content; // variable content
};

struct push_stream {
  std::byte magic[4];        // 'Z','P','\0','\1'
  std::uint32_t version;     // VERSION default =1
  std::byte reserved[16];    // reserved: full zero
  std::int64_t entry_length; // entry_length < 0 metadata; entry_length >0 blob; entry_length==0 end
  object_entry entry;        // object hash and content
  /* ... */
  std::uint64_t entry_end;      // ==>0000
  std::byte crc64_checksum[16]; // 16 byte CRC64 (ISO) checksum
};
```



推送协议采用 pktline 进行编码，用于展示进度以及结果，如果返回了字符串行 `unpack ok\nok branch`则表示分支更新成功。

服务端更新引用需要进行以下判断：

+ 如果远程的分支/标签不存在，那么 `old revision`则为全零。
+ 分支存在是否为保护分支。
+ 用户是否有相关权限。

服务端还要具备如下约束：

+ 更新引用前，元数据/Blob 应当先写入到（如未实现高可用的小文件存储，且以 DB/OSS 为后端） DB/OSS。

在 Push 过程中，服务端会将状态使用 pktline 编码进行返回，使用 pktline 解码后，为状态 + 信息，关键字如下：

| 关键字 | 用途 |
| --- | --- |
| rate | 表示当前进度 |
| unpack | 返回 ok 或者错误信息，意味着 unpack 成功或者失败<br/>格式：<br/>+ 成功：unpack ok<br/>+ 失败：unpack message |
| status | 服务端发送的一个状态，用户直接打印出来即可，如果本地是终端，责服务端可以输出彩色状态<br/>格式：status message |
| ng | 表示服务端拒绝更新引用。<br/>格式：ng refname reason |
| ok | 表示服务端接受更新引用。<br/>格式：ok refname newRev |


可选功能：我们还支持 `push-option` 功能，客户端可以设置 `X-Zeta-Push-Option-Count (ZETA_PUSH_OPTION_COUNT)` 和 `X-Zeta-Push-Option-${N} (ZETA_PUSH_OPTION_${N})` 以传递 `push-option`，平台可以定义一些自定义能力。


## 四、用户体验补充
在本章，我们将引入一些约定用于提高 zeta 工具和服务端数据传输之间的用户体验。

### 4.1 区域语言感知
在 HTTP 协议中，拥有标准字段 `Accept-Language`字段，浏览器请求时会将用户本地的语言设置传递到服务端，服务端可以根据用户的设置按照特定的语言返回，我们在实现 HugeSCM 服务端/客户端的时候也可以将本地环境变量的 LANG 解析成 Accept-Language 的字段，发送到服务端，从而按照用户的语言返回特定的信息，针对不同的协议，该传递的信息如下

+ HTTP 协议可以解析 `LANG`设置到 `Accept-Language`。
+ SSH 协议可以传输环境变量 `LANG`。

### 4.2 终端感知
客户端可以感知 zeta 是否运行在终端环境中，告知服务端，服务端可以据此是否开启更丰富的输出结果/

+ HTTP 协议可以将 `TERM`设置到 `X-Zeta-Terminal`。
+ SSH 协议可以传输环境变量 `TERM`。

