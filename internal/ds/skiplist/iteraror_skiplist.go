package skiplist

import (
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/iterator"
)

type SkiplistIterator struct {
	node        *Node
	lsnSnapshot constants.LsnType
	valid       bool
}

func (skiplist *Skiplist) NewSkiplistIterator(lsnSnapshot constants.LsnType) iterator.Iterator {
	return &SkiplistIterator{
		node:        skiplist.head,
		lsnSnapshot: lsnSnapshot,
		valid:       false,
	}
}

func (it *SkiplistIterator) Seek(key []byte) {

}

func (it *SkiplistIterator) Next() {

}

func (it *SkiplistIterator) Valid() bool {
	return false
}

func (it *SkiplistIterator) Key() []byte {
	return it.node.key
}

func (it *SkiplistIterator) Value() []byte {
	if it.Valid() {
		versionSize := len(it.node.versions)
		for i := versionSize - 1; i >= 0; i-- {
			if it.lsnSnapshot >= it.node.versions[i].lsn {
				if it.node.versions[i].entryType == constants.DelEntry {
					return nil
				} else {
					return it.node.versions[i].value
				}
			}
		}
	}
	return nil //not throwing error may need to return error if some case appears
}
