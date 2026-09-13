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
		baseMemtable: NewBaseMemtable(),
		skiplist:     skiplist.NewSkiplist(0.5, 12)}
}

func (memSkiplist *MemSkiplist) Get(key []byte, lsn constants.LsnType) []byte {
	memSkiplist.RegisterReader(lsn)
	defer memSkiplist.DeregisterReader(lsn)
	return memSkiplist.skiplist.Get(key, lsn)
}

func (memSkiplist *MemSkiplist) Put(key []byte, value []byte, lsn constants.LsnType, entryType constants.EntryType) []byte {
	memSkiplist.skiplist.Put(key, value, lsn, constants.PutEntry)
	return nil
}

func (memSkiplist *MemSkiplist) NewIterator(lsnSnapshot constants.LsnType) iterator.Iterator {
	return memSkiplist.skiplist.NewSnapshotIterator(lsnSnapshot)
}

func (memSkiplist *MemSkiplist) NewVersionedIterator() iterator.VersionedIterator {
	return memSkiplist.skiplist.NewFlushIterator()
}

func (memSkiplist *MemSkiplist) RegisterReader(lsn constants.LsnType) {
	memSkiplist.baseMemtable.RegisterReader(lsn)
}

func (memSkiplist *MemSkiplist) DeregisterReader(lsn constants.LsnType) {
	memSkiplist.baseMemtable.activeReaders.DeregisterReader(lsn)
}

func (MemSkiplist *MemSkiplist) Size() int {
	return MemSkiplist.skiplist.Size()
}

func (memSkiplist *MemSkiplist) Freeze() {
	memSkiplist.baseMemtable.status = IMMUTABLE
}

func (memSkiplist *MemSkiplist) IncrActiveWriter() {
	memSkiplist.baseMemtable.IncrActiveWriter()
}

func (memSkiplist *MemSkiplist) DecrActiveWriter() {
	memSkiplist.baseMemtable.DecrActiveWriter()
}
