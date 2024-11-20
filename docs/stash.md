# Stash

zeta stash 用于暂存修改，这既包括暂存 index，也包括暂存工作树。

在 Git 中，stash 的策略是：
1. 将 index 创建一个提交 A，A 的 parents 为 HEAD，其 tree 为 index 的 tree。
2. 创建一个合并提交 B，其父提交是 A 和 HEAD ，其 tree 为 worktree 的修改。

还原时，index 如果 HEAD 的 commit 没有改变，皆大欢喜。如果发生了改变，则需要支持合并流程，即产生新的 A/B，如果有冲突，则 stash pop 失败，如果没有冲突，则还原成合并的工作区。