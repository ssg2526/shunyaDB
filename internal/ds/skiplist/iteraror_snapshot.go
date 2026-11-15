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
		head: skiplist.head,
		node: skiplist.head,
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
	it.versionIdx = 0
}

func (it *SkipListIterator) Next() {
	if it.Valid() {
		if len(it.node.versions) > it.versionIdx {
			it.versionIdx++
		} else {
			it.node = it.node.lvlPtrs[0]
		}
	}
}

func (it *SkipListIterator) First() {

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
	// for i := len(it.node.versions) - 1; i >= 0; i-- {
	// 	if it.lsnSnapshot >= it.node.versions[i].lsn {
	// 		return it.node.versions[i].value
	// 	}
	// }
	// return nil //not throwing error may need to return error if some case appears
}

// func (it *SkipListIterator) IsVisible() bool {
// 	if it.lsnSnapshot >= it.node.versions[it.versionIdx].lsn {
// 		return it.node.versions[it.versionIdx].entryType != constants.DelEntry
// 	}
// 	return false
// }

func (it *SkipListIterator) Close() {

}

// func (it *SkipListIterator) skipInvisibleNodes() {
// 	for it.Valid() && !it.IsVisible() {
// 		it.node = it.node.lvlPtrs[0]
// 	}
// }
