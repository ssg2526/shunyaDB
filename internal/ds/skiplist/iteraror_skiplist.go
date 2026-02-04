package skiplist

import (
	"bytes"

	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/iterator"
)

type SkiplistIterator struct {
	head        *Node // to be checked if this is needed later
	node        *Node
	lsnSnapshot constants.LsnType
}

func (skiplist *Skiplist) NewSkiplistIterator(lsnSnapshot constants.LsnType) iterator.Iterator {
	return &SkiplistIterator{
		head:        skiplist.head,
		node:        skiplist.head,
		lsnSnapshot: lsnSnapshot,
	}
}

func (it *SkiplistIterator) Valid() bool {
	if it == nil || it.node == nil {
		return false
	}
	return true
}

func (it *SkiplistIterator) Seek(key []byte) {
	n := it.head

	for i := len(n.lvlPtrs) - 1; i >= 0; i-- {
		for n.lvlPtrs[i] != nil && bytes.Compare(n.lvlPtrs[i].key, key) < 0 {
			n = n.lvlPtrs[i]
		}
	}
	it.node = n.lvlPtrs[0]
	it.skipInvisibleNodes()
}

func (it *SkiplistIterator) Next() {
	if it.Valid() {
		it.node = it.node.lvlPtrs[0]
		it.skipInvisibleNodes()
	}
}

func (it *SkiplistIterator) Key() []byte {
	if it.Valid() {
		return it.node.key
	}
	return nil
}

func (it *SkiplistIterator) Value() []byte {
	if !it.Valid() {
		return nil
	}
	for i := len(it.node.versions) - 1; i >= 0; i-- {
		if it.lsnSnapshot >= it.node.versions[i].lsn {
			return it.node.versions[i].value
		}
	}
	return nil //not throwing error may need to return error if some case appears
}

func (it *SkiplistIterator) isVisible(node *Node) bool {
	for i := len(node.versions) - 1; i >= 0; i-- {
		if it.lsnSnapshot >= node.versions[i].lsn {
			return node.versions[i].entryType != constants.DelEntry
		}
	}
	return false
}

func (it *SkiplistIterator) skipInvisibleNodes() {
	for it.Valid() && !it.isVisible(it.node) {
		it.node = it.node.lvlPtrs[0]
	}
}
