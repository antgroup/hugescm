# Viewport

基于 [github.com/robinovitch61/viewport](https://github.com/robinovitch61/viewport) 的 TUI viewport 组件。

## 来源

- 原项目: https://github.com/robinovitch61/viewport
- License: MIT (见 LICENSE 文件)
- 拷贝时间: 2026-04-17

## 改动说明

1. 包路径修改为 `github.com/antgroup/hugescm/modules/viewport`
2. 简化目录结构（移除 `internal/` 层级）
3. 类型重命名避免冲突（`Option` → `FuzzyOption`）
4. 测试文件修复
5. 扩展 PageDown 键绑定：支持 `space` 键翻页（原项目仅支持 `pgdown`、`f`、`ctrl+f`）

## 功能特性

- 支持滚动、翻页、跳转
- 支持文本换行和水平滚动
- 支持 item 选择
- 支持搜索和高亮 (filterableviewport)
- 支持自定义键绑定
- 支持 ANSI 颜色和 Unicode

## 目录结构

```
modules/viewport/
├── fuzzy/                    # 模糊搜索
├── item/                     # Item 接口实现
├── filterableviewport/       # 过滤功能
├── viewport.go               # 核心实现
├── keymap.go                 # 键盘映射
├── styles.go                 # 样式定义
└── test_util.go              # 测试工具
```

## 使用示例

```go
import "github.com/antgroup/hugescm/modules/viewport"

// 定义对象类型
type Line struct {
    content string
    item    viewport.Item
}

func (l Line) GetItem() viewport.Item { return l.item }

// 创建 viewport
vp := viewport.New[Line](width, height,
    viewport.WithSelectionEnabled[Line](true),
    viewport.WithWrapText[Line](true),
)
```