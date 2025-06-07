package memtable

import (
	"github.com/ssg2526/shunya/internal/ds/skiplist"
)

type Memtable struct {
	skiplist  *skiplist.Skiplist
	sizeBytes int
}

func NewMemtable() *Memtable {
	return &Memtable{skiplist: skiplist.NewSkiplist(0.5, 12)}
}

func (memtable *Memtable) Get(key string) string {
	return memtable.skiplist.Get(key)
}

func (memtable *Memtable) Insert(key string, value string) {
	memtable.skiplist.Insert(key, value)
}

func (memtable *Memtable) Delete(key string) bool {
	return memtable.skiplist.Delete(key)
}
