package main

import "fmt"

type TreeEntry struct {
	Name string
	OID  string
}

func compareTrees(original, a, b []*TreeEntry) (aChanges, bChanges, conflicts []string) {
	entryMap := make(map[string][3]*TreeEntry) // 用于存储三个状态的 map

	// 记录原始状态
	for _, entry := range original {
		entryMap[entry.Name] = [3]*TreeEntry{entry, nil, nil}
	}
	// 记录 a 状态的变更
	for _, entry := range a {
		e := entryMap[entry.Name]
		e[1] = entry
		entryMap[entry.Name] = e
	}
	// 记录 b 状态的变更
	for _, entry := range b {
		e := entryMap[entry.Name]
		e[2] = entry
		entryMap[entry.Name] = e
	}

	// 对比变更
	for name, entries := range entryMap {
		o, a, b := entries[0], entries[1], entries[2]

		// 检测删除和修改的冲突
		if (a == nil && o != nil && b != nil && b.OID != o.OID) || (b == nil && o != nil && a != nil && a.OID != o.OID) {
			conflicts = append(conflicts, fmt.Sprintf("Conflict: %s", name))
			continue // 继续下一个条目的检测
		}

		// 检测修改的冲突
		if a != nil && b != nil && a.OID != b.OID && (o == nil || (a.OID != o.OID && b.OID != o.OID)) {
			conflicts = append(conflicts, fmt.Sprintf("Conflict: %s", name))
			continue // 继续下一个条目的检测
		}

		// 记录 a 的变更（如果有）
		if a != nil && (o == nil || a.OID != o.OID) {
			aChanges = append(aChanges, fmt.Sprintf("Modified: %s", name))
		} else if a == nil && o != nil {
			aChanges = append(aChanges, fmt.Sprintf("Deleted: %s", name))
		}

		// 记录 b 的变更（如果有）
		if b != nil && (o == nil || b.OID != o.OID) {
			bChanges = append(bChanges, fmt.Sprintf("Modified: %s", name))
		} else if b == nil && o != nil {
			bChanges = append(bChanges, fmt.Sprintf("Deleted: %s", name))
		}
	}

	return aChanges, bChanges, conflicts
}
func main() {
	// 示例数据
	original := []*TreeEntry{
		{"file1", "oid123"},
		{"file2", "oid456"},
		{"file5", "oid567"},
		{"file6", "oid567"},
	}
	a := []*TreeEntry{
		{"file1", "oid999"}, // 修改了 file1
		{"file2", "oid789"}, // 修改了 file2
		{"file3", "oidabc"}, // 新增 file3
		{"file5", "oid678"},
		{"file6", "oid678"},
		{"file7", "oid7777"},
	}
	b := []*TreeEntry{
		{"file1", "oid999"}, // 修改了 file1
		{"file2", "oid456"},
		{"file4", "oiddef"}, // 新增 file4
		{"file5", "oid789"},
		{"file7", "oid8888"},
	}

	aChanges, bChanges, conflicts := compareTrees(original, a, b)

	fmt.Println("Changes in a:", aChanges)
	fmt.Println("Changes in b:", bChanges)
	fmt.Println("Conflicts:", conflicts)
}
