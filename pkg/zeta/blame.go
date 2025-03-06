// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

// BlameResult represents the result of a Blame operation.
type BlameResult struct {
	// Path is the path of the File that we're blaming.
	Path string
	// Rev (Revision) is the hash of the specified Commit used to generate this result.
	Rev plumbing.Hash
	// Lines contains every line with its authorship.
	Lines []*Line
}

func contentLines(content string) []string {
	splits := strings.Split(content, "\n")
	// remove the last line if it is empty
	if splits[len(splits)-1] == "" {
		return splits[:len(splits)-1]
	}
	return splits
}

// Blame returns a BlameResult with the information about the last author of
// each line from file `path` at commit `c`.
func Blame(ctx context.Context, c *object.Commit, path string) (*BlameResult, error) {
	// The file to blame is identified by the input arguments:
	// commit and path. commit is a Commit object obtained from a Repository. Path
	// represents a path to a specific file contained in the repository.
	//
	// Blaming a file is done by walking the tree in reverse order trying to find where each line was last modified.
	//
	// When a diff is found it cannot immediately assume it came from that commit, as it may have come from 1 of its
	// parents, so it will first try to resolve those diffs from its parents, if it couldn't find the change in its
	// parents then it will assign the change to itself.
	//
	// When encountering 2 parents that have made the same change to a file it will choose the parent that was merged
	// into the current branch first (this is determined by the order of the parents inside the commit).
	//
	// This currently works on a line by line basis, if performance becomes an issue it could be changed to work with
	// hunks rather than lines. Then when encountering diff hunks it would need to split them where necessary.

	b := new(blame)
	b.fRev = c
	b.path = path
	b.q = new(priorityQueue)

	file, err := b.fRev.File(ctx, path)
	if err != nil {
		return nil, err
	}
	contents, err := file.UnifiedText(ctx, false)
	if err != nil {
		return nil, err
	}
	finalLines := contentLines(contents)
	finalLength := len(finalLines)

	needsMap := make([]lineMap, finalLength)
	for i := range needsMap {
		needsMap[i] = lineMap{i, i, nil, -1}
	}
	b.q.Push(&queueItem{
		nil,
		nil,
		c,
		path,
		contents,
		needsMap,
		0,
		false,
		0,
	})
	items := make([]*queueItem, 0)
	for {
		items = items[:0]
		for {
			if b.q.Len() == 0 {
				return nil, errors.New("invalid state: no items left on the blame queue")
			}
			item := b.q.Pop()
			items = append(items, item)
			next := b.q.Peek()
			if next == nil || next.Hash != item.Commit.Hash {
				break
			}
		}
		finished, err := b.addBlames(ctx, items)
		if err != nil {
			return nil, err
		}
		if finished {
			break
		}
	}

	b.lineToCommit = make([]*object.Commit, finalLength)
	for i := range needsMap {
		b.lineToCommit[i] = needsMap[i].Commit
	}

	lines, err := newLines(finalLines, b.lineToCommit)
	if err != nil {
		return nil, err
	}

	return &BlameResult{
		Path:  path,
		Rev:   c.Hash,
		Lines: lines,
	}, nil
}

// Line values represent the contents and author of a line in BlamedResult values.
type Line struct {
	// Author is the email address of the last author that modified the line.
	Author string
	// AuthorName is the name of the last author that modified the line.
	AuthorName string
	// Text is the original text of the line.
	Text string
	// Date is when the original text of the line was introduced
	Date time.Time
	// Hash is the commit hash that introduced the original line
	Hash plumbing.Hash
}

func newLine(author, authorName, text string, date time.Time, hash plumbing.Hash) *Line {
	return &Line{
		Author:     author,
		AuthorName: authorName,
		Text:       text,
		Hash:       hash,
		Date:       date,
	}
}

func newLines(contents []string, commits []*object.Commit) ([]*Line, error) {
	result := make([]*Line, 0, len(contents))
	for i := range contents {
		result = append(result, newLine(
			commits[i].Author.Email, commits[i].Author.Name, contents[i],
			commits[i].Author.When, commits[i].Hash,
		))
	}

	return result, nil
}

// this struct is internally used by the blame function to hold its
// inputs, outputs and state.
type blame struct {
	// the path of the file to blame
	path string
	// the commit of the final revision of the file to blame
	fRev *object.Commit
	// resolved lines
	lineToCommit []*object.Commit
	// queue of commits that need resolving
	q *priorityQueue
}

type lineMap struct {
	Orig, Cur    int
	Commit       *object.Commit
	FromParentNo int
}

func (b *blame) addBlames(ctx context.Context, curItems []*queueItem) (bool, error) {
	curItem := curItems[0]

	// Simple optimisation to merge paths, there is potential to go a bit further here and check for any duplicates
	// not only if they are all the same.
	if len(curItems) == 1 {
		curItems = nil
	} else if curItem.IdenticalToChild {
		allSame := true
		lenCurItems := len(curItems)
		lowestParentNo := curItem.ParentNo
		for i := 1; i < lenCurItems; i++ {
			if !curItems[i].IdenticalToChild || curItem.Child != curItems[i].Child {
				allSame = false
				break
			}
			lowestParentNo = min(lowestParentNo, curItems[i].ParentNo)
		}
		if allSame {
			curItem.Child.numParentsNeedResolving = curItem.Child.numParentsNeedResolving - lenCurItems + 1
			curItems = nil // free the memory
			curItem.ParentNo = lowestParentNo

			// Now check if we can remove the parent completely
			for curItem.Child.IdenticalToChild && curItem.Child.MergedChildren == nil && curItem.Child.numParentsNeedResolving == 1 {
				oldChild := curItem.Child
				curItem.Child = oldChild.Child
				curItem.ParentNo = oldChild.ParentNo
			}
		}
	}

	// if we have more than 1 item for this commit, create a single needsMap
	if len(curItems) > 1 {
		curItem.MergedChildren = make([]childToNeedsMap, len(curItems))
		for i, c := range curItems {
			curItem.MergedChildren[i] = childToNeedsMap{c.Child, c.NeedsMap, c.IdenticalToChild, c.ParentNo}
		}
		newNeedsMap := make([]lineMap, 0, len(curItem.NeedsMap))
		newNeedsMap = append(newNeedsMap, curItems[0].NeedsMap...)

		for i := 1; i < len(curItems); i++ {
			cur := curItems[i].NeedsMap
			n := 0 // position in newNeedsMap
			c := 0 // position in current list
			for c < len(cur) {
				if n == len(newNeedsMap) {
					newNeedsMap = append(newNeedsMap, cur[c:]...)
					break
				} else if newNeedsMap[n].Cur == cur[c].Cur {
					n++
					c++
				} else if newNeedsMap[n].Cur < cur[c].Cur {
					n++
				} else {
					newNeedsMap = append(newNeedsMap, cur[c])
					newPos := len(newNeedsMap) - 1
					for newPos > n {
						newNeedsMap[newPos-1], newNeedsMap[newPos] = newNeedsMap[newPos], newNeedsMap[newPos-1]
						newPos--
					}
				}
			}
		}
		curItem.NeedsMap = newNeedsMap
		curItem.IdenticalToChild = false
		curItem.Child = nil
		//curItems = nil // free the memory
	}

	parents, err := parentsContainingPath(ctx, curItem.path, curItem.Commit)
	if err != nil {
		return false, err
	}

	anyPushed := false
	for parnetNo, prev := range parents {
		currentHash, err := blobHash(ctx, curItem.path, curItem.Commit)
		if err != nil {
			return false, err
		}
		prevHash, err := blobHash(ctx, prev.Path, prev.Commit)
		if err != nil {
			return false, err
		}
		if currentHash == prevHash {
			if len(parents) == 1 && curItem.MergedChildren == nil && curItem.IdenticalToChild {
				// commit that has 1 parent and 1 child and is the same as both, bypass it completely
				b.q.Push(&queueItem{
					Child:            curItem.Child,
					Commit:           prev.Commit,
					path:             prev.Path,
					Contents:         curItem.Contents,
					NeedsMap:         curItem.NeedsMap, // reuse the NeedsMap as we are throwing away this item
					IdenticalToChild: true,
					ParentNo:         curItem.ParentNo,
				})
			} else {
				b.q.Push(&queueItem{
					Child:            curItem,
					Commit:           prev.Commit,
					path:             prev.Path,
					Contents:         curItem.Contents,
					NeedsMap:         append([]lineMap(nil), curItem.NeedsMap...), // create new slice and copy
					IdenticalToChild: true,
					ParentNo:         parnetNo,
				})
				curItem.numParentsNeedResolving++
			}
			anyPushed = true
			continue
		}

		// get the contents of the file
		file, err := prev.Commit.File(ctx, prev.Path)
		if err != nil {
			return false, err
		}
		prevContents, err := file.UnifiedText(ctx, false)
		if err != nil {
			return false, err
		}
		u, err := diferenco.DoUnified(ctx, &diferenco.Options{
			S1: prevContents,
			S2: curItem.Contents,
		})
		if err != nil {
			return false, err
		}
		prevl := -1
		curl := -1
		need := 0
		getFromParent := make([]lineMap, 0)
	out:
		for _, h := range u.Hunks {
			for hl := range h.Lines {
				switch h.Lines[hl].Kind {
				case diferenco.Equal:
					prevl++
					curl++
					if curl == curItem.NeedsMap[need].Cur {
						// add to needs
						getFromParent = append(getFromParent, lineMap{curl, prevl, nil, -1})
						// move to next need
						need++
						if need >= len(curItem.NeedsMap) {
							break out
						}
					}
				case diferenco.Insert:
					curl++
					if curl == curItem.NeedsMap[need].Cur {
						// the line we want is added, it may have been added here (or by another parent), skip it for now
						need++
						if need >= len(curItem.NeedsMap) {
							break out
						}
					}
				case diferenco.Delete:
					prevl++
					continue out
				default:
					return false, errors.New("invalid state: invalid hunk Type")
				}
			}
		}

		if len(getFromParent) > 0 {
			b.q.Push(&queueItem{
				curItem,
				nil,
				prev.Commit,
				prev.Path,
				prevContents,
				getFromParent,
				0,
				false,
				parnetNo,
			})
			curItem.numParentsNeedResolving++
			anyPushed = true
		}
	}

	curItem.Contents = "" // no longer need, free the memory

	if !anyPushed {
		return finishNeeds(curItem)
	}

	return false, nil
}

func finishNeeds(curItem *queueItem) (bool, error) {
	// any needs left in the needsMap must have come from this revision
	for i := range curItem.NeedsMap {
		if curItem.NeedsMap[i].Commit == nil {
			curItem.NeedsMap[i].Commit = curItem.Commit
			curItem.NeedsMap[i].FromParentNo = -1
		}
	}

	if curItem.Child == nil && curItem.MergedChildren == nil {
		return true, nil
	}

	if curItem.MergedChildren == nil {
		return applyNeeds(curItem.Child, curItem.NeedsMap, curItem.IdenticalToChild, curItem.ParentNo)
	}

	for _, ctn := range curItem.MergedChildren {
		m := 0 // position in merged needs map
		p := 0 // position in parent needs map
		for p < len(ctn.NeedsMap) {
			if ctn.NeedsMap[p].Cur == curItem.NeedsMap[m].Cur {
				ctn.NeedsMap[p].Commit = curItem.NeedsMap[m].Commit
				m++
				p++
			} else if ctn.NeedsMap[p].Cur < curItem.NeedsMap[m].Cur {
				p++
			} else {
				m++
			}
		}
		finished, err := applyNeeds(ctn.Child, ctn.NeedsMap, ctn.IdenticalToChild, ctn.ParentNo)
		if finished || err != nil {
			return finished, err
		}
	}

	return false, nil
}

func applyNeeds(child *queueItem, needsMap []lineMap, identicalToChild bool, parentNo int) (bool, error) {
	if identicalToChild {
		for i := range child.NeedsMap {
			l := &child.NeedsMap[i]
			if l.Cur != needsMap[i].Cur || l.Orig != needsMap[i].Orig {
				return false, errors.New("needsMap isn't the same? Why not??")
			}
			if l.Commit == nil || parentNo < l.FromParentNo {
				l.Commit = needsMap[i].Commit
				l.FromParentNo = parentNo
			}
		}
	} else {
		i := 0
	out:
		for j := range child.NeedsMap {
			l := &child.NeedsMap[j]
			for needsMap[i].Orig < l.Cur {
				i++
				if i == len(needsMap) {
					break out
				}
			}
			if l.Cur == needsMap[i].Orig {
				if l.Commit == nil || parentNo < l.FromParentNo {
					l.Commit = needsMap[i].Commit
					l.FromParentNo = parentNo
				}
			}
		}
	}
	child.numParentsNeedResolving--
	if child.numParentsNeedResolving == 0 {
		finished, err := finishNeeds(child)
		if finished || err != nil {
			return finished, err
		}
	}

	return false, nil
}

// String prints the results of a Blame using git-blame's style.
func (b BlameResult) String() string {
	var buf bytes.Buffer

	// max line number length
	mlnl := len(strconv.Itoa(len(b.Lines)))
	// max author length
	mal := b.maxAuthorLength()
	format := fmt.Sprintf("%%s (%%-%ds %%s %%%dd) %%s\n", mal, mlnl)

	for ln := range b.Lines {
		_, _ = fmt.Fprintf(&buf, format, b.Lines[ln].Hash.String()[:8],
			b.Lines[ln].AuthorName, b.Lines[ln].Date.Format("2006-01-02 15:04:05 -0700"), ln+1, b.Lines[ln].Text)
	}
	return buf.String()
}

// utility function to calculate the number of runes needed
// to print the longest author name in the blame of a file.
func (b BlameResult) maxAuthorLength() int {
	m := 0
	for ln := range b.Lines {
		m = max(m, utf8.RuneCountInString(b.Lines[ln].AuthorName))
	}
	return m
}

type childToNeedsMap struct {
	Child            *queueItem
	NeedsMap         []lineMap
	IdenticalToChild bool
	ParentNo         int
}

type queueItem struct {
	Child                   *queueItem
	MergedChildren          []childToNeedsMap
	Commit                  *object.Commit
	path                    string
	Contents                string
	NeedsMap                []lineMap
	numParentsNeedResolving int
	IdenticalToChild        bool
	ParentNo                int
}

type priorityQueueImp []*queueItem

func (pq *priorityQueueImp) Len() int { return len(*pq) }
func (pq *priorityQueueImp) Less(i, j int) bool {
	return !(*pq)[i].Commit.Less((*pq)[j].Commit)
}
func (pq *priorityQueueImp) Swap(i, j int) { (*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i] }
func (pq *priorityQueueImp) Push(x any)    { *pq = append(*pq, x.(*queueItem)) }
func (pq *priorityQueueImp) Pop() any {
	n := len(*pq)
	ret := (*pq)[n-1]
	(*pq)[n-1] = nil // ovoid memory leak
	*pq = (*pq)[0 : n-1]

	return ret
}
func (pq *priorityQueueImp) Peek() *object.Commit {
	if len(*pq) == 0 {
		return nil
	}
	return (*pq)[0].Commit
}

type priorityQueue priorityQueueImp

func (pq *priorityQueue) Init()    { heap.Init((*priorityQueueImp)(pq)) }
func (pq *priorityQueue) Len() int { return (*priorityQueueImp)(pq).Len() }
func (pq *priorityQueue) Push(c *queueItem) {
	heap.Push((*priorityQueueImp)(pq), c)
}
func (pq *priorityQueue) Pop() *queueItem {
	return heap.Pop((*priorityQueueImp)(pq)).(*queueItem)
}
func (pq *priorityQueue) Peek() *object.Commit { return (*priorityQueueImp)(pq).Peek() }

type parentCommit struct {
	Commit *object.Commit
	Path   string
}

func parentsContainingPath(ctx context.Context, path string, c *object.Commit) ([]parentCommit, error) {
	// TODO: benchmark this method making git.object.Commit.parent public instead of using
	// an iterator
	var result []parentCommit
	iter := c.MakeParents()
	for {
		parent, err := iter.Next(ctx)
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		if _, err := parent.File(ctx, path); err == nil {
			result = append(result, parentCommit{parent, path})
		}
	}
}

func blobHash(ctx context.Context, path string, commit *object.Commit) (plumbing.Hash, error) {
	file, err := commit.File(ctx, path)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return file.Hash, nil
}
