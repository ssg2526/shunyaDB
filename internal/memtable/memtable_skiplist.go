package memtable

import (
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/ds/skiplist"
	"github.com/ssg2526/shunya/internal/iterator"
)

type MemSkiplist struct {
	baseMemtable *BaseMemtable
	skiplist     *skiplist.Skiplist
}

func NewMemSkiplist() *MemSkiplist {
	return &MemSkiplist{
		baseMemtable: &BaseMemtable{status: MUTABLE},
		skiplist:     skiplist.NewSkiplist(0.5, 12)}
}

func (memSkiplist *MemSkiplist) Get(key []byte, lsn constants.LsnType) []byte {
	return memSkiplist.skiplist.Get(key, lsn)
}

func (memSkiplist *MemSkiplist) Put(key []byte, value []byte, lsn constants.LsnType, entryType constants.EntryType) []byte {
	memSkiplist.skiplist.Put(key, value, lsn, constants.PutEntry)
	return nil
}

func (memSkiplist *MemSkiplist) NewIterator(lsnSnapshot constants.LsnType) iterator.Iterator {
	return memSkiplist.skiplist.NewSkiplistIterator(lsnSnapshot)
}

func (MemSkiplist *MemSkiplist) Size() int {
	return MemSkiplist.skiplist.Size()
}
