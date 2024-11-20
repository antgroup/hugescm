# HugeSCM 稀疏检出实现原理

在 HugeSCM 中，我们实现了非常高效的稀疏检出机制，并能在忽略文件名大小写的系统中避免文件名冲突导致的数据丢失。这该如何实现？

## 实现原理

在 HugeSCM 中，我们引入了 `noder.Matcher` 接口，该接口定义如下

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

对于稀疏检出，我们的策略是将路径转为 noder.Matcher，然后从 root tree 开始匹配，对于非 tree 对象则检出，tree 对象如果未匹配上，则跳过，匹配到则使用其子 Matcher，如果子 Matcher 为 nil 或长度为 0，则直接跳过匹配，认为所有的子条目均能匹配上，这样一样就建立了稀疏树。

我们对 TreeNode/Index Node/Filesyem Node 均采用相同的机制过滤，就能够实现稀疏树之外的目录不可见，状态不会跟踪。但存在一个问题，HugeSCM 借鉴了 Git，使用 index 机制创建提交，代码基本源自 go-git，而 go-git 的实现不太完美，不够好，无法支持全功能的稀疏检出，实现也有诸多错误，我们的改造量也比较大，因此，在 HugeSCM 中，我们引入了不可变对象的概念，将稀疏树的排除目录作为不可变条目，在写入 tree 时合并这些条目。就可以达到相应的目的。

在 Windows/macOS 的系统上，由于其文件系统忽略路径大小写，如果存储库中有忽略大小写后同名的文件/目录，则可能会导致工作区异常，在 git 中，这个问题一直存在，也基本无法得到解决，而 HugeSCM 利用稀疏检出的机制，将冲突的路径视为不可变，不可见对象，在 Windows/macOS 上对其保持不检出，不能被修改的原则，避免了同名文件的数据丢失。