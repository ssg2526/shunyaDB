package storage

import (
	"bytes"
	"context"
	"encoding/binary"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maypok86/otter/v2"
	"github.com/ssg2526/shunya/config"
	constants "github.com/ssg2526/shunya/internal/constants"
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
	ctx             context.Context
	cancel          context.CancelFunc
	blockCache      *otter.Cache[BlockCacheKey, sstable.SSTBlock]
	sstIndexCache   *otter.Cache[uint64, sstable.SSTIndex]
	openSstCache    *otter.Cache[uint64, *sstable.SSTableRead]
}

type BlockCacheKey struct {
	sstFileNum  uint64
	blockOffset uint32
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

	openSstCache, err := otter.New(&otter.Options[uint64, *sstable.SSTableRead]{
		MaximumSize: config.ShunyaConfigs.OpenSstLRUSize,
		OnDeletion: func(e otter.DeletionEvent[uint64, *sstable.SSTableRead]) {
			onSstOpenFileCacheEvict(e.Value)
		},
	})

	if err != nil {
		// log error
	}

	sstIndexCache, err := otter.New(&otter.Options[uint64, sstable.SSTIndex]{
		MaximumWeight: uint64(config.ShunyaConfigs.SSTIndexCacheSizeByte),
		Weigher: func(key uint64, value sstable.SSTIndex) uint32 {
			return value.IndexSize
		},
	})
	if err != nil {
		// log error
	}

	blockCache, err := otter.New(&otter.Options[BlockCacheKey, sstable.SSTBlock]{
		MaximumWeight: uint64(config.ShunyaConfigs.BlockCacheSizeByte),
		Weigher: func(key BlockCacheKey, value sstable.SSTBlock) uint32 {
			return value.BlockLen
		},
	})

	if err != nil {
		//log error
	}

	ctx, cancel := context.WithCancel(context.Background())
	storage := &Storage{
		activeMem:       memtableObj,
		flushQueue:      make(chan memtable.Memtable, config.ShunyaConfigs.FlushQueueSize),
		idleFlushTicker: time.NewTicker(time.Duration(config.ShunyaConfigs.IdleFlushIntervalMillis) * time.Millisecond),
		wal:             wal.InitWal(),
		manifest:        manifest,
		openSstCache:    openSstCache,
		blockCache:      blockCache,
		sstIndexCache:   sstIndexCache,
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

func (storage *Storage) sstBlockLoader(ctx context.Context, blockCacheKey BlockCacheKey) (sstable.SSTBlock, error) {
	sstableRead, err := storage.openSstCache.Get(ctx, blockCacheKey.sstFileNum,
		otter.LoaderFunc[uint64, *sstable.SSTableRead](storage.sstReadFileLoader))
	if err != nil {
		return sstable.SSTBlock{}, err
	}

	return sstableRead.LoadBlock(sstableRead.SstFile, blockCacheKey.blockOffset)
}

func (storage *Storage) sstIndexLoader(ctx context.Context, sstFileNum uint64) (sstable.SSTIndex, error) {
	sstableRead, err := storage.openSstCache.Get(ctx, sstFileNum,
		otter.LoaderFunc[uint64, *sstable.SSTableRead](storage.sstReadFileLoader))
	if err != nil {
		return sstable.SSTIndex{}, err
	}

	sstIndex, err := sstableRead.LoadIndex(sstableRead.SstFile)
	if err != nil {
		return sstable.SSTIndex{}, err
	}
	return sstIndex, nil
}

func (storage *Storage) sstReadFileLoader(ctx context.Context, sstFileNum uint64) (*sstable.SSTableRead, error) {
	return sstable.OpenSSTableRead(sstFileNum)
}

func onSstOpenFileCacheEvict(ssTableRead *sstable.SSTableRead) {
	if ssTableRead.GetSSTReadRef() == 0 {
		ssTableRead.CloseSST()
	} else {
		// push the file to a queue for closing
	}
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
			data, lookupResult = curMtQ[i].Get(key, lsn)
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

			value, result, err := storage.getFromSst(sstMeta.sstFileNum, key, lsn)
			if err != nil {
				// TODO: surface read errors; skipping this file could return a stale value from an older one
				return []byte("")
			}
			switch result {
			case constants.KeyFound:
				return value
			case constants.KeyDeleted:
				return []byte("")
			}
		}
	}

	return []byte("")
}

func (storage *Storage) getFromSst(sstFileNum uint64, key []byte, lsn constants.LsnType) ([]byte, constants.LookupResult, error) {
	sstIndex, err := storage.sstIndexCache.Get(storage.ctx, sstFileNum,
		otter.LoaderFunc[uint64, sstable.SSTIndex](storage.sstIndexLoader))
	if err != nil {
		return nil, constants.KeyNotFound, err
	}

	indexEntry, ok := sstIndex.FindBlock(key, lsn)
	if !ok {
		return nil, constants.KeyNotFound, nil
	}

	blockCacheKey := BlockCacheKey{
		sstFileNum:  sstFileNum,
		blockOffset: indexEntry.BlockOffset,
	}
	sstBlock, err := storage.blockCache.Get(storage.ctx, blockCacheKey,
		otter.LoaderFunc[BlockCacheKey, sstable.SSTBlock](storage.sstBlockLoader))
	if err != nil {
		return nil, constants.KeyNotFound, err
	}

	value, result := storage.binarySearchInSstBlock(&sstBlock, key, lsn)
	return value, result, nil
}

func (storage *Storage) Range(key []byte, limit int, lsnSnapshot constants.LsnType) [][]byte {
	it := storage.activeMem.NewIterator(lsnSnapshot)
	res := [][]byte{}
	it.Seek(key)
	for it.Valid() && limit > 0 {
		key := it.Key()
		val := it.Value()
		keyLen := len(key)
		valLen := len(val)
		kvBuf := make([]byte, len(key)+len(val)+8)
		binary.LittleEndian.PutUint32(kvBuf, uint32(keyLen))
		binary.LittleEndian.PutUint32(kvBuf, uint32(valLen))
		copy(kvBuf[8:], key)
		copy(kvBuf[8+keyLen:], val)
		res = append(res, kvBuf)
		it.Next()
		limit--
	}
	return res
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

func (storage *Storage) binarySearchInSstBlock(sstBlock *sstable.SSTBlock, key []byte, lsn constants.LsnType) ([]byte, constants.LookupResult) {
	return sstBlock.Search(key, lsn)
}
