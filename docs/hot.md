# hot - Git 存储库维护工具

`hot` 是整合到 HugeSCM 中的 Git 存储库维护工具，专用于存储库治理和优化。它帮助开发者高效地清理、维护和迁移 Git 存储库。

---

## 为什么需要 hot？

Git 存储库在长期使用中会积累技术债务：

| 挑战 | hot 解决方案 |
|------|-------------|
| 历史中的敏感数据 | `hot remove` 重写历史，彻底删除敏感信息 |
| 存储库膨胀 | `hot size`/`hot smart` 识别并清理大文件 |
| SHA1 安全问题 | `hot mc` 迁移到 SHA256 对象格式 |
| 过期分支/标签 | `hot prune-refs`/`hot expire-refs` 自动清理 |
| 开源发布准备 | `hot unbranch` 创建干净的公开历史 |

---

## 命令概览

| 命令 | 描述 |
|------|------|
| `hot size` | 查看存储库大小和大文件（原始大小） |
| `hot az` | 分析大文件的近似压缩大小 |
| `hot remove` | 删除存储库中的文件并重写历史 |
| `hot smart` | 交互式清理大文件（结合 `size` 和 `remove` 命令） |
| `hot graft` | 交互式清理大文件（嫁接模式） |
| `hot mc` | 迁移存储库对象格式（SHA1 ↔ SHA256） |
| `hot unbranch` | 线性化存储库历史（移除合并提交） |
| `hot prune-refs` | 按前缀清理引用 |
| `hot scan-refs` | 扫描本地存储库中的引用 |
| `hot expire-refs` | 清理过期引用 |
| `hot snapshot` | 为工作树创建快照提交 |
| `hot cat` | 查看存储库对象（commit/tree/tag/blob） |
| `hot stat` | 查看存储库状态 |
| `hot co` | 克隆存储库（实验性） |

---

## 常见使用场景

### 1. 查找大文件

```shell
# 查看大文件的原始大小
hot size

# 查看近似压缩大小
hot az

# 交互模式，筛选 >= 20MB 的文件
hot smart -L20m
```

### 2. 删除敏感数据

误提交了密码、密钥等敏感信息时，使用 `hot remove` 彻底删除：

```shell
# 删除指定文件并重写历史
hot remove path/to/secret.txt

# 使用通配符删除
hot remove "*.env" --confirm --prune

# 删除后清理
hot remove sensitive.txt --prune
git reflog expire --expire=now --all
git gc --prune=now --aggressive
```

**注意**：重写历史后，需要强制推送（`git push --force`），并通知协作者重新克隆。

### 3. 迁移对象格式

从 SHA1 迁移到 SHA256（推荐，提升安全性）：

```shell
# 迁移远程存储库
hot mc https://github.com/user/repo.git

# 迁移本地存储库
hot mc /path/to/repo --format sha256
```

迁移过程会：
1. 克隆原存储库
2. 转换所有对象到新格式
3. 生成新的存储库目录

### 4. 清理过期引用

长期开发的存储库会积累大量过期分支和标签：

```shell
# 先扫描引用
hot scan-refs

# 按前缀删除引用
hot prune-refs "feature/deprecated-"

# 删除超过 90 天未更新的引用
hot expire-refs --days 90

# 仅删除分支
hot expire-refs --days 90 --branches

# 仅删除标签
hot expire-refs --days 90 --tags
```

### 5. 线性化历史

用于开源发布或简化历史：

```shell
# 移除所有合并提交，使历史线性化
hot unbranch --confirm

# 创建保留最近历史的孤儿分支（适用于开源场景）
hot unbranch -K1 master -T new-branch

# 保留最近 10 次提交
hot unbranch -K10 main -T clean-history
```

**选项说明**：
- `-K N`：保留最近 N 次提交
- `-T <branch>`：指定新分支名称
- `--confirm`：确认执行

### 6. 查看对象

调试和分析存储库对象：

```shell
# 以 JSON 格式查看 commit/tree/tag
hot cat HEAD --json

# 查看文件内容
hot cat HEAD:README.md

# 查看二进制文件（16 进制显示）
hot cat HEAD:docs/images/blob.png

# 查看特定对象
hot cat abc123def456
```

### 7. 创建快照

快速保存当前工作状态：

```shell
# 创建快照提交
hot snapshot -m "WIP: 功能开发中"

# 带标签的快照
hot snapshot -m "Release candidate" --tag v1.0.0-rc1
```

---

## 高级用法

### 交互式大文件清理

`hot smart` 提供交互式界面，逐步清理大文件：

```shell
# 启动交互模式
hot smart

# 指定最小文件大小
hot smart -L50m  # 仅显示 >= 50MB 的文件

# 自动模式（跳过确认）
hot smart --auto
```

### 嫁接模式清理

`hot graft` 使用嫁接（graft）技术，无需重写完整历史：

```shell
# 嫁接模式清理
hot graft path/to/large-file.bin

# 从特定提交开始嫁接
hot graft large.bin --since abc123
```

嫁接模式比 `remove` 更快，但会改变提交 ID。

### 查看存储库状态

```shell
# 查看整体状态
hot stat

# 显示详细信息
hot stat --verbose
```

---

## 注意事项

### 重写历史的风险

使用 `hot remove`、`hot unbranch` 等命令会重写 Git 历史：

1. **提交 ID 会改变**：所有受影响提交的 SHA 都会变化
2. **需要强制推送**：必须使用 `git push --force`
3. **协作者需重新克隆**：其他人需要重新克隆存储库
4. **备份重要分支**：操作前建议创建备份分支

### 性能建议

对于大型存储库（>10GB）：

```shell
# 先分析，再清理
hot size > large-files.txt
hot smart -L100m

# 分批清理
hot remove "path/to/large1.bin"
git gc --prune=now
hot remove "path/to/large2.bin"
git gc --prune=now
```

---

## 示例场景

### 场景 1：开源前清理

准备将内部项目开源：

```shell
# 1. 线性化历史，保留最近提交
hot unbranch -K50 main -T public

# 2. 删除敏感配置文件
hot remove "config/prod/*" --prune
hot remove ".env.*" --prune

# 3. 清理大文件
hot smart -L10m

# 4. 迁移到 SHA256
hot mc /path/to/repo --format sha256
```

### 场景 2：存储库瘦身

存储库过大，需要瘦身：

```shell
# 1. 分析存储库
hot size
hot az

# 2. 交互式清理
hot smart -L20m

# 3. 清理过期分支
hot expire-refs --days 180 --branches

# 4. 清理过期标签
hot expire-refs --days 365 --tags

# 5. 最终清理
git reflog expire --expire=now --all
git gc --prune=now --aggressive
```

### 场景 3：安全加固

修复 SHA1 碰撞风险：

```shell
# 1. 检查当前格式
hot stat

# 2. 迁移到 SHA256
hot mc https://internal.example.com/repo.git

# 3. 验证新存储库
cd new-repo
hot stat
git log --oneline | head
```

---

## 获取帮助

每个命令都有详细的帮助信息：

```shell
hot -h              # 查看所有命令
hot size -h         # 查看 size 命令帮助
hot remove -h       # 查看 remove 命令帮助
```