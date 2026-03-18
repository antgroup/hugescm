# HugeSCM 稀疏检出

稀疏检出（Sparse Checkout）允许用户只检出存储库中的部分目录，而非完整的工作区。这对于巨型存储库特别有用，可以显著减少本地存储空间和检出时间。

## 一、概述

### 1.1 什么是稀疏检出

在传统的版本控制系统中，检出（checkout/clone）意味着获取存储库的完整内容。但在巨型存储库中，这往往是不必要的：

- AI 模型存储库可能包含多个模型的多个版本
- 游戏存储库可能包含大量美术资源
- 单体仓库可能包含多个子项目

稀疏检出允许用户只获取需要的目录，而不是整个存储库。

### 1.2 HugeSCM 稀疏检出的优势

| 特性 | 说明 |
|------|------|
| 按需获取 | 仅下载指定目录的元数据和文件 |
| 节省空间 | 大幅减少本地磁盘占用 |
| 快速检出 | 减少网络传输，加快检出速度 |
| 冲突处理 | 自动处理文件名大小写冲突 |

## 二、基本用法

### 2.1 检出时指定目录

使用 `checkout` 命令的 `-s` 或 `--sparse` 选项：

```bash
# 检出单个目录
zeta checkout http://zeta.example.io/group/repo myrepo -s src/core

# 检出多个目录
zeta checkout http://zeta.example.io/group/repo myrepo -s src/core -s src/utils

# 使用简写
zeta co http://zeta.example.io/group/repo myrepo -s dir1
```

### 2.2 查看当前稀疏配置

```bash
# 查看稀疏检出配置
zeta config core.sparse

# 查看配置文件
cat .zeta/zeta.toml
```

### 2.3 修改稀疏配置

修改稀疏配置需要通过修改配置文件实现：

```bash
# 修改配置文件中的 core.sparse 项
# 编辑 .zeta/zeta.toml 文件：
# [core]
# sparse = ["src/core", "src/utils", "src/newdir"]
```

### 2.4 应用稀疏配置

修改配置后，重新检出或切换分支来应用：

```bash
# 切换到其他分支再切回来
zeta switch other-branch
zeta switch mainline

# 或者恢复工作区
zeta restore .
```

## 三、命令详解

### 3.1 checkout 命令的稀疏选项

```bash
zeta checkout [options] <url> [<directory>]

稀疏相关选项:
  -s, --sparse=<dir>,...   指定稀疏检出的目录（可多次使用）
  -L, --limit=<size>       限制检出文件大小
  --one                    逐一检出模式
```

### 3.2 完整选项

| 选项 | 说明 |
|------|------|
| `-b, --branch=<branch>` | 检出后创建指定分支 |
| `-t, --tag=<tag>` | 检出特定标签 |
| `--commit=<commit>` | 检出特定提交 |
| `-s, --sparse=<dir>` | 稀疏检出目录 |
| `-L, --limit=<size>` | 限制检出文件大小 |
| `--depth=<n>` | 浅表检出深度 |
| `--one` | 逐一检出大文件 |
| `--batch` | 批量检出文件 |
| `--snapshot` | 检出不可编辑的快照 |
| `--quiet` | 静默模式 |

## 四、配置文件

### 4.1 稀疏配置存储

稀疏配置存储在 `.zeta/zeta.toml` 文件中：

```toml
[core]
remote = "https://zeta.example.io/group/repo"
sparse = ["src/core", "src/utils"]
compression-algo = "zstd"
```

### 4.2 配置格式说明

- `sparse` 是一个字符串数组
- 每个元素是一个目录路径（相对于仓库根目录）
- 路径不需要以 `/` 开头

## 五、实现原理

### 5.1 Matcher 接口

在 HugeSCM 中，我们引入了 `noder.Matcher` 接口来实现稀疏匹配：

```go
type Matcher interface {
	Len() int
	Match(name string) (Matcher, bool)
}

type sparseTreeMatcher struct {
	entries map[string]*sparseTreeMatcher
}

func (m *sparseTreeMatcher) Len() int {
	return len(m.entries)
}

func (m *sparseTreeMatcher) Match(name string) (Matcher, bool) {
	sm, ok := m.entries[name]
	return sm, ok
}

func (m *sparseTreeMatcher) insert(p string) {
	dv := strengthen.StrSplitSkipEmpty(p, '/', 10)
	current := m
	for _, d := range dv {
		e, ok := current.entries[d]
		if !ok {
			e = &sparseTreeMatcher{entries: make(map[string]*sparseTreeMatcher)}
			current.entries[d] = e
		}
		current = e
	}
}

func NewSparseTreeMatcher(dirs []string) Matcher {
	root := &sparseTreeMatcher{entries: make(map[string]*sparseTreeMatcher)}
	for _, d := range dirs {
		root.insert(d)
	}
	return root
}
```

### 5.2 匹配策略

稀疏检出的匹配策略：

1. 将路径转为 `noder.Matcher`
2. 从 root tree 开始匹配
3. 对于非 tree 对象则检出
4. tree 对象如果未匹配上，则跳过
5. 匹配到则使用其子 Matcher
6. 如果子 Matcher 为 nil 或长度为 0，则跳过匹配，检出所有子条目

### 5.3 不可变对象机制

HugeSCM 使用 index 机制创建提交，为支持全功能稀疏检出，引入了**不可变对象**的概念：

- 将稀疏树的排除目录作为不可变条目
- 在写入 tree 时合并这些条目
- 保证提交时包含完整的目录结构

### 5.4 文件名大小写冲突处理

在 Windows/macOS 系统上，文件系统忽略文件名大小写，可能导致同名文件冲突：

```
src/File.txt
src/file.txt  # Windows/macOS 上会冲突
```

HugeSCM 的解决方案：

1. 检测同名冲突文件
2. 将冲突路径视为不可变、不可见对象
3. 在 Windows/macOS 上不检出这些文件
4. 避免数据丢失问题

## 六、使用场景

### 6.1 AI 模型开发

```bash
# 只检出特定模型的目录
zeta co http://zeta.example.io/ai/models mymodels -s gpt-4 -s bert

# 只检出训练脚本，不检出模型文件
zeta co http://zeta.example.io/ai/project myproject -s scripts -s configs
```

### 6.2 单体仓库开发

```bash
# 只检出自己负责的子项目
zeta co http://zeta.example.io/mono monorepo -s services/auth -s libs/common
```

### 6.3 文档贡献

```bash
# 只检出文档目录
zeta co http://zeta.example.io/project proj -s docs -s README.md
```

### 6.4 CI/CD 构建

```bash
# 只检出构建所需的目录
zeta co http://zeta.example.io/project proj -s src -s build -s package.json
```

## 七、与 Git 的差异

### 7.1 Git 稀疏检出

```bash
# Git 需要多步操作
git clone --filter=blob:none --sparse http://example.io/repo
cd repo
git sparse-checkout init --cone
git sparse-checkout set dir1 dir2
```

### 7.2 HugeSCM 稀疏检出

```bash
# HugeSCM 一条命令搞定
zeta co http://zeta.example.io/repo myrepo -s dir1 -s dir2
```

### 7.3 主要差异

| 特性 | Git | HugeSCM |
|-----|-----|---------|
| 配置复杂度 | 多步操作 | 一条命令 |
| 服务端支持 | 部分过滤 | 原生支持 |
| 元数据获取 | 全量 | 按需 |
| 大小写冲突 | 无处理 | 自动处理 |
| 子命令 | `sparse-checkout add/set/list` | 通过配置修改 |

## 八、最佳实践

### 8.1 初始检出

```bash
# 建议：先稀疏检出，再按需添加目录
zeta co http://zeta.example.io/repo myrepo -s src/core

# 后续如需添加目录，修改配置文件后重新检出
# 编辑 .zeta/zeta.toml 添加目录
# 然后执行 switch 或 restore
```

### 8.2 配合按需获取

```bash
# 稀疏检出 + 按需获取
zeta co http://zeta.example.io/repo myrepo -s src --limit=0

# 需要特定文件时再检出
zeta checkout -- path/to/file
```

### 8.3 避免频繁修改

频繁修改稀疏配置会导致：
- 频繁的网络请求
- 工作区文件的删除和下载

建议：
- 初始时规划好需要的目录
- 批量修改后再应用

## 九、故障排查

### 9.1 文件未检出

```bash
# 检查稀疏配置
zeta config core.sparse

# 确认目录是否在配置中
# 如不在，修改配置后重新检出
```

### 9.2 配置不生效

```bash
# 检查配置文件
cat .zeta/zeta.toml

# 确认配置格式正确
```

### 9.3 稀疏配置丢失

```bash
# 检查配置文件是否正确
zeta config core.sparse

# 重新设置
zeta config core.sparse '["dir1", "dir2"]'
```

## 十、相关命令

| 命令 | 说明 |
|-----|------|
| `zeta checkout` | 检出存储库 |
| `zeta config` | 查看和修改配置 |
| `zeta restore` | 恢复工作区文件 |
| `zeta switch` | 切换分支 |
| `zeta status` | 查看工作区状态 |