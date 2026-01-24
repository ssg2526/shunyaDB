package memtable

import (
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/ds/skiplist"
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

func (memSkiplist *MemSkiplist) Get(key string) string {
	return memSkiplist.skiplist.Get(key)
}

func (memSkiplist *MemSkiplist) Put(key string, value string, lsn constants.LsnType, entryType constants.EntryType) string {
	memSkiplist.skiplist.Put(key, value, lsn, constants.PutEntry)
	return ""
}

func (MemSkiplist *MemSkiplist) Size() int {
	return MemSkiplist.skiplist.Size()
}
