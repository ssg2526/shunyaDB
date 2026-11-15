package storage

import (
	"bytes"
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ssg2526/shunya/config"
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/iterator"
	"github.com/ssg2526/shunya/internal/memtable"
	"github.com/ssg2526/shunya/internal/sstable"
	"github.com/ssg2526/shunya/internal/wal"
)

type Storage struct {
	mu              sync.Mutex
	mtQPtr          atomic.Pointer[[]memtable.Memtable]
	activeMem       memtable.Memtable
	flushQueue      chan memtable.Memtable
	idleFlushTicker *time.Ticker
	wal             *wal.WAL
	manifest        *Manifest
	cacheManager    *sstable.CacheManager
	ctx             context.Context
	cancel          context.CancelFunc
}

const (
	MEM_TABLE_FLUSH_SIZE = 417792
)

func (storage *Storage) AppendToWal(commandData []byte) constants.LsnType {
	return storage.wal.AppendToWal(commandData)
}

func InitStorage() (*Storage, error) {
	var memtableObj memtable.Memtable

	switch config.ShunyaConfigs.MemTableType {
	case "skiplist":
		memtableObj = memtable.NewMemtable(memtable.SKIPLIST)
	default:
		memtableObj = memtable.NewMemtable(memtable.SKIPLIST)
	}

	manifest, err := InitManifest()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	storage := &Storage{
		activeMem:       memtableObj,
		flushQueue:      make(chan memtable.Memtable, config.ShunyaConfigs.FlushQueueSize),
		idleFlushTicker: time.NewTicker(time.Duration(config.ShunyaConfigs.IdleFlushIntervalMillis) * time.Millisecond),
		wal:             wal.InitWal(),
		manifest:        manifest,
		cacheManager:    sstable.InitCacheManager(),
		ctx:             ctx,
		cancel:          cancel,
	}
	initialMtQ := []memtable.Memtable{memtableObj}
	storage.mtQPtr.Store(&initialMtQ)

	storage.StartFlushLoop()
	go storage.StartIdleFlushWorker(time.Duration(config.ShunyaConfigs.IdleFlushIntervalMillis) * time.Millisecond)

	if err := storage.RestoreData(); err != nil {
		return nil, err
	}

	return storage, nil
}

func (storage *Storage) RestoreData() error {
	return storage.wal.ReplayWal(storage.manifest.GetMaxLsn(), func(lsn constants.LsnType, data []byte) error {
		op, key, value := DecodeWalCommand(data)
		if op == constants.DelEntry {
			storage.Del(key, lsn)
		} else {
			storage.Put(key, value, lsn)
		}
		return nil
	})
}

func (storage *Storage) addMemTable() memtable.Memtable {
	var memtableObj memtable.Memtable

	switch config.ShunyaConfigs.MemTableType {
	case "skiplist":
		memtableObj = memtable.NewMemtable(memtable.SKIPLIST)
	default:
		memtableObj = memtable.NewMemtable(memtable.SKIPLIST)
	}
	oldMtQ := *storage.mtQPtr.Load()
	newMtQ := append(slices.Clone(oldMtQ), memtableObj)
	storage.mtQPtr.Store(&newMtQ)
	return memtableObj
}

func (storage *Storage) Get(key []byte, lsn constants.LsnType) []byte {
	data, lookupResult := storage.activeMem.Get(key, lsn)

	if lookupResult != constants.KeyNotFound {
		return data
	}

	curMtQ := *storage.mtQPtr.Load()
	for i := len(curMtQ) - 1; i >= 0; i-- {
		if curMtQ[i] != storage.activeMem {
			data, lookupResult = curMtQ[i].Get(key, lsn) //TODO: USE iterators and refactor this
			if lookupResult != constants.KeyNotFound {
				return data
			}
		}
	}

	version := storage.manifest.AcquireRef()
	defer storage.manifest.ReleaseRef(version)

	for lvl, level := range version.Levels {
		for i := range level {
			sstMeta := level[i]
			if lvl == 0 {
				sstMeta = level[len(level)-1-i]
			}
			if bytes.Compare(key, sstMeta.minKey) < 0 || bytes.Compare(key, sstMeta.maxKey) > 0 || sstMeta.minLsn > lsn {
				continue
			}

			sstIterator := sstable.NewIterator(storage.ctx, sstMeta.sstFileNum, storage.cacheManager)
			defer sstIterator.Close()

			sstIterator.SeekGE(key, lsn)

			if sstIterator.Valid() {
				return sstIterator.Value()
			}
		}
	}

	return nil
}

func (storage *Storage) Range(startKey []byte, endKey []byte, limit int, lsnSnapshot constants.LsnType) [][]byte {
	manifestVersion := storage.manifest.AcquireRef()
	iteratorArraySize := 1
	for _, level := range manifestVersion.Levels {
		if len(level) > 0 {
			iteratorArraySize += len(level)
		}
	}

	iteratorArray := make([]iterator.Iterator, iteratorArraySize)
	it := storage.activeMem.NewRawIterator()
	iteratorArray[0] = it

	var wg sync.WaitGroup

	for _, level := range manifestVersion.Levels {
		wg.Go(func() {
			storage.buildIteratorForKey(level, startKey, lsnSnapshot, iteratorArray)
		})
	}

	wg.Wait()

	vIt := iterator.NewVisibleIterator(iterator.NewMergeIterator(iteratorArray), lsnSnapshot)

	values := make([][]byte, 0, 100)
	vIt.First()

	for vIt.Valid() && len(values) <= min(limit, 100) {
		values = append(values, vIt.Value())
		vIt.Next()
	}
	// TODO: build continuation token as well

	return values
}

// TODO: need to implememt soft throttling and hard throttling
func (storage *Storage) Put(key []byte, value []byte, lsn constants.LsnType) []byte {
	storage.mu.Lock()
	storage.checkMemTableForFreeze()
	defer storage.mu.Unlock()
	targetMem := storage.activeMem
	targetMem.Put(key, value, lsn, constants.PutEntry)
	return []byte("OK")
}

func (storage *Storage) Del(key []byte, lsn constants.LsnType) []byte {
	storage.mu.Lock()
	storage.checkMemTableForFreeze()
	defer storage.mu.Unlock()
	targetMem := storage.activeMem
	targetMem.Put(key, nil, lsn, constants.DelEntry)
	return []byte("OK")
}

func (storage *Storage) checkMemTableForFreeze() {
	if storage.activeMem.Size() > MEM_TABLE_FLUSH_SIZE {
		storage.activeMem.Freeze()
		memToflush := storage.activeMem
		storage.activeMem = storage.addMemTable()
		storage.mu.Unlock()
		storage.flushQueue <- memToflush
		storage.mu.Lock()
	}
}

func (storage *Storage) buildIteratorForKey(level []SSTableMeta, key []byte, lsn constants.LsnType, iterArr []iterator.Iterator) {

}
