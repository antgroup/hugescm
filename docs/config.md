# HugeSCM 支持的配置和环境变量备忘录

| 配置 | 环境变量 | 备注 |
| :--- | :--- | :--- |
| `core.sharingRoot` | `ZETA_CORE_SHARING_ROOT` | 该配置用于指定 blob 存储的 root，以表明用户试图在多个存储库中复用 blob 对象，用来降低磁盘存储。 |
| `core.sparse`||存储库稀疏检出配置，用户不应设置在全局修改该配置，可以在存储库级别修改|
| `core.remote`||存储库远程设置，通常不宜修改此配置|
| `user.name`<br/> | `ZETA_AUTHOR_NAME` | 作者名 |
| | `ZETA_COMMITTER_NAME` | 提交者 |
| `user.email`<br/> | `ZETA_AUTHOR_EMAIL` | 作者邮箱 |
| | `ZETA_COMMITTER_EMAIL` | 提交者邮箱 |
|  | `ZETA_AUTHOR_DATE` | 作者签名时间 |
|  | `ZETA_COMMITTER_DATE` | 提交时间 |
| `core.accelerator` | `ZETA_CORE_ACCELERATOR` | 下载加速器 |
| `core.optimizeStrategy` | `ZETA_CORE_OPTIMIZE_STRATEGY` | 存储库空间管理策略 |
| `core.concurrenttransfers` | `ZETA_CORE_CONCURRENT_TRANSFERS` | 大文件下载时的并发策略 |
|  | `ZETA_CORE_PROMISOR` | 按需下载标志（默认开启，设置为 false 时关闭） |
| `core.editor` | `ZETA_EDITOR`，兼容 `GIT_EDITOR/EDITOR` | 提交信息/标签编辑时的编辑器，默认会搜索 VIM 等。 |
|  | `ZETA_MERGE_TEXT_DRIVER` | 文本合并工具，未设置时是使用内置实现，可以设置为 `git`，设置后我们将使用 git merge-file 来做三路合并。 |
|  | `ZETA_SSL_NO_VERIFY` | 是否禁用 SSL 验证 |
| `http.sslVerify` |  | 与 `ZETA_SSL_NO_VERIFY`刚好相反，只有显式设置为 false 才会禁用 SSL 验证。 |
| `http.extraHeader` |  | 设置 HTTP 附加头，如果设置了 `Authorization`，客户端会跳过自身的权限预验证，该机制可以实现一些自定义的能力。 |
| `transport.maxEntries`|`ZETA_TRANSPORT_MAX_ENTRIES`|设置 batch 下载对象数量限制，影响 batch 接口调用次数|
| `transport.largeSize`|`ZETA_TRANSPORT_LARGE_SIZE`|设置大文件大小，默认 10M|
| `transport.externalProxy`|`ZETA_TRANSPORT_EXTERNAL_PROXY`|为 `direct` 下载配置外部代理|
| `diff.algorithm`||diff 算法，支持：`histogram`，`onp`，`myers`，`patience`，`minimal`|
| `merge.conflictStyle`||指定在合并时将存在冲突的代码块写入工作区文件的样式，支持 `merge`，`diff3`，`zdiff3`|
||`ZETA_PAGER/PAGER`|终端分页工具，未设置时会搜索 less 命令行，可设置 `PAGER=""` 禁用分页，Windows 下请安装 less for windows。|
|  | `ZETA_TERMINAL_PROMPT` | 显式设置 false 时禁用终端，此时客户端就失去 CUI 交互能力。 |
