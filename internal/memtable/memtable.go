package memtable

import (
	"sync"
	"sync/atomic"

	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/iterator"
)

type Memtable interface {
	Get(key []byte, lsn constants.LsnType) []byte
	Put(key []byte, value []byte, lsn constants.LsnType, entryType constants.EntryType) []byte
	NewIterator(snapshotLSN constants.LsnType) iterator.Iterator
	Size() int
	UpdateToFlushPending()
	// UpdateToFlushing()
	IncrActiveWriter()
	DecrActiveWriter()
}

type MemTableType uint8
type MemTableStatus uint8

const (
	MUTABLE MemTableStatus = iota
	FLUSHPENDING
	FLUSHING
)

const (
	SKIPLIST MemTableType = iota
)

type BaseMemtable struct {
	status        MemTableStatus
	activeWriters atomic.Int32
	activeReaders *ReaderTracker
}

type ReaderTracker struct {
	mu         sync.Mutex
	lsnReaders map[constants.LsnType]int
	minLsn     constants.LsnType
}

func NewBaseMemtable() *BaseMemtable {
	return &BaseMemtable{
		status: MUTABLE,
		activeReaders: &ReaderTracker{
			lsnReaders: make(map[constants.LsnType]int),
			minLsn:     ^constants.LsnType(0),
		},
	}
}

func (baseMemtable *BaseMemtable) RegisterReader(lsn constants.LsnType) {
	readerTracker := baseMemtable.activeReaders
	readerTracker.mu.Lock()
	defer readerTracker.mu.Unlock()
	readerTracker.lsnReaders[lsn]++
	if lsn < readerTracker.minLsn {
		readerTracker.minLsn = lsn
	}
}

func (readerTracker *ReaderTracker) DeregisterReader(lsn constants.LsnType) {
	readerTracker.mu.Lock()
	defer readerTracker.mu.Unlock()
	readerTracker.lsnReaders[lsn]--
	if readerTracker.lsnReaders[lsn] == 0 {
		delete(readerTracker.lsnReaders, lsn)
		if lsn == readerTracker.minLsn {
			readerTracker.calculateNewMinLsn()
		}
	}
}

func (readerTracker *ReaderTracker) calculateNewMinLsn() {
	minLsn := ^constants.LsnType(0)
	for lsn := range readerTracker.lsnReaders {
		if lsn < minLsn {
			readerTracker.minLsn = lsn
		}
	}
}

func NewMemtable(mtType MemTableType) Memtable {
	switch mtType {
	case SKIPLIST:
		return NewMemSkiplist()
	default:
		return NewMemSkiplist()
	}
}
