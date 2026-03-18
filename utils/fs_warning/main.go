// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var (
	fsNameHighlight = lipgloss.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipgloss.Color("#D70000"), Dark: lipgloss.Color("#FF6B6B"),
	}).Bold(true)
)

// 模拟真正的 warn 函数
func warn(format string, a ...any) {
	var b bytes.Buffer
	_, _ = b.WriteString("warning: ")
	fmt.Fprintf(&b, format, a...)
	_ = b.WriteByte('\n')
	_, _ = os.Stderr.Write(b.Bytes())
}

func main() {
	fmt.Println("=== 文件系统警告模拟 ===")
	fmt.Println()

	// 模拟 checkout 场景
	warn("Checking out to a network filesystem '%s' may cause data corruption or performance issues.", fsNameHighlight.Render("NFS"))
	fmt.Println()

	// 模拟 open 场景
	warn("The repository on network filesystem '%s' may have data corruption or performance issues.", fsNameHighlight.Render("Ceph"))
	fmt.Println()

	// 模拟其他文件系统
	warn("Checking out to a network filesystem '%s' may cause data corruption or performance issues.", fsNameHighlight.Render("SMB"))
	fmt.Println()

	fmt.Println("=== 原始文本（无颜色） ===")
	fmt.Printf("warning: Checking out to a network filesystem 'NFS' may cause data corruption or performance issues.\n")
}
