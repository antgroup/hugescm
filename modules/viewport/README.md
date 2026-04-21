# Viewport

An advanced terminal viewport component for [Bubble Tea](https://github.com/charmbracelet/bubbletea) terminal UI (TUI) applications.

This is a fork of [github.com/robinovitch61/viewport](https://github.com/robinovitch61/viewport) integrated into the zeta project.

## Overview

The viewport module provides a feature-rich terminal viewport component for building interactive TUI applications. It offers advanced text display capabilities including wrapping, scrolling, selection, and filtering.

## Features

### Core Viewport

- **Text wrapping** - Toggleable text wrapping with horizontal panning for unwrapped lines
- **ANSI & Unicode support** - Full support for ANSI escape codes and Unicode characters
- **Item selection** - Individual item selection with customizable styling
- **Sticky scrolling** - Auto-follow new content with sticky top/bottom scrolling
- **Sticky header** - Configurable sticky header that remains visible while scrolling
- **Highlight ranges** - Highlight specific text ranges with custom styles
- **Content saving** - Save viewport content to file
- **Efficient concatenation** - Efficient item concatenation via `MultiItem` (e.g., prefixing line numbers)

### Filterable Viewport

The `filterableviewport` package extends the core viewport with:

- **Multiple filter modes** - Exact, regex, case-insensitive (built-in); custom modes supported
- **Match highlighting** - Highlighted matches with focused/unfocused styles
- **Match navigation** - Next/previous match navigation
- **Matches-only view** - Hide non-matching items
- **Match limiting** - Configurable match limit for large content
- **Search history** - Browse previous searches (up/down arrow while editing)

## Installation

This module is part of the zeta project and is located at `github.com/antgroup/hugescm/modules/viewport`.

## Usage

### Basic Viewport

Implement the `Object` interface on your type:

```go
import (
    "github.com/antgroup/hugescm/modules/viewport"
    "github.com/antgroup/hugescm/modules/viewport/item"
)

type myObject struct {
    item item.Item
}

func (o myObject) GetItem() item.Item {
    return o.item
}
```

Create a viewport and set content:

```go
vp := viewport.New[myObject](
    width, height,
    viewport.WithSelectionEnabled[myObject](true),
    viewport.WithWrapText[myObject](true),
)

objects := []myObject{
    {item: item.NewItem("first line")},
    {item: item.NewItem("second line")},
}

vp.SetObjects(objects)
```

Wire it into your Bubble Tea model's `Update` and `View`:

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    m.viewport, cmd = m.viewport.Update(msg)
    return m, cmd
}

func (m model) View() string {
    return m.viewport.View()
}
```

### Filterable Viewport

Wrap an existing viewport to add filtering:

```go
import "github.com/antgroup/hugescm/modules/viewport/filterableviewport"

fvp := filterableviewport.New[myObject](
    vp,
    filterableviewport.WithPrefixText[myObject]("Filter:"),
    filterableviewport.WithEmptyText[myObject]("No Current Filter"),
    filterableviewport.WithMatchingItemsOnly[myObject](false),
    filterableviewport.WithCanToggleMatchingItemsOnly[myObject](true),
)

fvp.SetObjects(objects)
```

### Custom Filter Modes

Define custom filter logic with a `FilterMode`:

```go
import (
    "strings"

    "charm.land/bubbles/v2/key"
    "github.com/antgroup/hugescm/modules/viewport/filterableviewport"
    "github.com/antgroup/hugescm/modules/viewport/item"
)

const FilterPrefix filterableviewport.FilterModeName = "prefix"

prefixMode := filterableviewport.FilterMode{
    Name:  FilterPrefix,
    Key:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prefix filter")),
    Label: "[prefix]",
    GetMatchFunc: func(filterText string) (filterableviewport.MatchFunc, error) {
        return func(content string) []item.ByteRange {
            if strings.HasPrefix(content, filterText) {
                return []item.ByteRange{{Start: 0, End: len(filterText)}}
            }
            return nil
        }, nil
    },
}
```

## Default Key Bindings

### Viewport Navigation

| Key | Action |
|---|---|
| `j` / `down` / `enter` | Scroll down |
| `k` / `up` | Scroll up |
| `f` / `pgdown` / `ctrl+f` / `space` | Page down |
| `b` / `pgup` / `ctrl+b` | Page up |
| `d` / `ctrl+d` | Half page down |
| `u` / `ctrl+u` | Half page up |
| `g` / `ctrl+g` / `home` | Jump to top |
| `G` / `end` | Jump to bottom |
| `left` / `right` | Horizontal pan |

> **Note**: The viewport does not handle quit keys (`q`, `esc`, `ctrl+c`) - this is intentional as viewport is a generic scrolling component and the quit logic should be handled by the parent application.

### Filterable Viewport

| Key | Action |
|---|---|
| `/` | Start exact filter |
| `r` | Start regex filter |
| `i` | Start case-insensitive filter |
| `enter` | Apply filter |
| `esc` | Cancel/clear filter |
| `n` | Next match |
| `N` (shift+n) | Previous match |
| `o` | Toggle matches-only view |
| `up` / `down` | Browse search history (while editing) |

## License

MIT License - See [LICENSE](LICENSE) file for details.

Original work Copyright (c) 2026 Leo Robinovitch