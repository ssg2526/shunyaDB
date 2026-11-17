package skiplist

import (
	"bytes"

	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/iterator"
)

type SkipListIterator struct {
	head       *Node // to be checked if this is needed later
	node       *Node
	versionIdx int
}

func (skiplist *Skiplist) NewSkipListIterator() iterator.Iterator {
	return &SkipListIterator{
		head:       skiplist.head,
		node:       skiplist.head,
		versionIdx: -1,
	}
}

func (it *SkipListIterator) Valid() bool {
	if it == nil || it.node == nil {
		return false
	}
	return true
}

func (it *SkipListIterator) SeekGE(key []byte, lsn constants.LsnType) {
	n := it.head

	for i := len(n.lvlPtrs) - 1; i >= 0; i-- {
		for n.lvlPtrs[i] != nil && bytes.Compare(n.lvlPtrs[i].key, key) < 0 {
			n = n.lvlPtrs[i]
		}
	}

	it.node = n.lvlPtrs[0]
	if it.node == nil {
		return
	}

	if bytes.Equal(it.node.key, key) && it.node.versions[0].lsn <= lsn {
		for i := len(it.node.versions) - 1; i >= 0; i-- {
			if it.node.versions[i].lsn <= lsn {
				it.versionIdx = i
				return
			}
		}
		it.node = n.lvlPtrs[0]
		if it.node == nil {
			return
		}
	}

	it.versionIdx = len(it.node.versions) - 1
}

func (it *SkipListIterator) Next() {
	if it.Valid() {
		if it.versionIdx > 0 {
			it.versionIdx--
			return
		}

		it.node = it.node.lvlPtrs[0]
		if it.node != nil {
			it.versionIdx = len(it.node.versions) - 1
		}
	}
}

func (it *SkipListIterator) First() {
	it.node = it.head.lvlPtrs[0]
	if it.node != nil {
		it.versionIdx = len(it.node.versions) - 1
	}
}

func (it *SkipListIterator) Key() constants.InternalKey {
	if it.Valid() {
		return constants.InternalKey{
			Key:       it.node.key,
			Lsn:       it.node.versions[it.versionIdx].lsn,
			EntryType: it.node.versions[it.versionIdx].entryType,
		}
	}
	return constants.InternalKey{}
}

func (it *SkipListIterator) Value() []byte {
	if !it.Valid() {
		return nil
	}

	return it.node.versions[it.versionIdx].value
}

func (it *SkipListIterator) Close() {
	it.node = nil
	it.head = nil
	it.versionIdx = -1
}
