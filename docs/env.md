# HugeSCM 支持的环境变量备忘录

## 客户端行为

+  `ZETA_TERMINAL_PROMPT` 该环境变量控制是否开启终端提示符，对于需要在命令行交互的（如输入用户名和密码），未设置此值会影响该功能。
+  `ZETA_TRACE` 开启 zeta 调试模式
+  `ZETA_EXTENSION_DRAGONFLY_GET` 设置 `dfget` 路径。
+  `ZETA_SHARING_ROOT` 设置 `core.shardingRoot` 目录，该配置存在时，zeta 会使用该变量设置的目录作为 BLOB 的存储目录。
+  `ZETA_TRANSPORT_MAX_ENTRIES` 指定批量下载对象一次性下载数量，默认 32000，用户可以修改。
+  `ZETA_TRANSPORT_LARGE_SIZE` 将指定大小的文件试为大文件，使用单个文件下载接口，默认为 10M，用户可以指定 `ZETA_TRANSPORT_LARGE_SIZE=512k`。

## 注入配置

请注意，通过环境修改的配置仅对当前进程有效，不会覆盖配置文件中的配置。

+  `ZETA_CORE_OPTIMIZE_STRATEGY` 存储库优化策略，支持 `heuristical`，`eager`，`extreme`，其中 `extreme` 模式会删除所有超过 50M 的 blob，该行为比较极端，多数情况无需开启。
+  `ZETA_CORE_ACCELERATOR` 存储库下载加速器，目前只支持 `dragonfly` （dragonfly P2P 加速）和 `aria2`（aria2 加速） 以及 `direct` （oss 直连下载，不走 zeta）， 可以使用 `ZETA_EXTENSION_DRAGONFLY_GET` 覆盖环境变量中的 dfget ，使用 `ZETA_EXTENSION_ARIA2C` 覆盖环境变量中的 `aria2c`，未设置 `ZETA_CORE_ACCELERATOR` 时，我们走 zeta 的大文件下载接口下载。
+  `ZETA_CORE_CONCURRENT_TRANSFERS` 临时覆盖 `core.concurrenttransfers` 配置，下载并发数，有效值是 `0-50`。
+  `ZETA_AUTHOR_NAME` commit 作者名称
+  `ZETA_AUTHOR_EMAIL` commit 作者邮箱
+  `ZETA_AUTHOR_DATE` commit 作者时间
+  `ZETA_COMMITTER_NAME` commit 提交者名称
+  `ZETA_COMMITTER_EMAIL` commit 提交者邮箱
+  `ZETA_COMMITTER_DATE` commit 提交者时间

Committer 未显示设置时会使用 Author 的值。