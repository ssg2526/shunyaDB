package sstable

import (
	"context"
	"fmt"

	"github.com/maypok86/otter/v2"
	constants "github.com/ssg2526/shunya/internal/constants"
)

type SSTableIterator struct {
	sstFileNum    uint64
	sstRead       *SSTableRead
	mode          constants.IteratorMode
	cacheManager  *CacheManager
	lsnSnapshot   constants.LsnType
	sstIndex      *SSTIndex
	sstBlock      *SSTBlock
	indexEntryPos int32
	blockEntryPos int32
	valid         bool
	ctx           context.Context
}

func NewIterator(ctx context.Context, sstFileNum uint64, lsnSnapshot constants.LsnType, cacheManager *CacheManager) *SSTableIterator {
	sstableIterator := &SSTableIterator{
		sstFileNum:   sstFileNum,
		lsnSnapshot:  lsnSnapshot,
		cacheManager: cacheManager,
		ctx:          ctx,
	}

	return sstableIterator
}

func (it *SSTableIterator) Valid() bool {
	return it.valid
}

func (it *SSTableIterator) Seek(key []byte) {
	it.valid = false
	if it.sstIndex == nil {
		it.loadSstableIndex()
	}

	if it.mode == constants.Point {
		indexEntryPos, ok := it.sstIndex.FindBlock(key, it.lsnSnapshot)
		if !ok {
			return
		}
		it.indexEntryPos = indexEntryPos
	} else if it.mode == constants.Range {
		indexEntryPos, ok := it.sstIndex.FindRangeStartBlock(key, it.lsnSnapshot)
		if !ok {
			return
		}
		it.indexEntryPos = indexEntryPos
	}

	blockCacheKey := BlockCacheKey{
		sstFileNum:  it.sstFileNum,
		blockOffset: it.sstIndex.IndexEntries[it.indexEntryPos].BlockOffset,
		blockLen:    it.sstIndex.IndexEntries[it.indexEntryPos].BlockLen,
	}

	sstBlock, err := it.cacheManager.BlockCache.Get(it.ctx, blockCacheKey,
		otter.LoaderFunc[BlockCacheKey, *SSTBlock](it.sstBlockLoader))

	if err != nil {
		// log error
		return
	}

	blockPos, ok := sstBlock.Search(key, it.lsnSnapshot)
	if ok == false {
		return
	}
	it.blockEntryPos = blockPos
	it.sstBlock = sstBlock
	it.valid = true
}

func (it *SSTableIterator) Next() {

}

func (it *SSTableIterator) Key() []byte {
	return it.sstBlock.Entries[it.blockEntryPos].internalKey.key
}

func (it *SSTableIterator) Value() []byte {
	if !it.Valid() {
		return nil
	}
	blockEntry := it.sstBlock.Entries[it.blockEntryPos]
	if blockEntry.internalKey.entryType == constants.DelEntry {
		return []byte("")
	}
	return blockEntry.value
}

func (it *SSTableIterator) Close() {
	if it.sstRead != nil {
		it.sstRead.Release()
		it.sstRead = nil
	}
}

func (it *SSTableIterator) sstFile(ctx context.Context) (*SSTableRead, error) {
	if it.sstRead != nil {
		return it.sstRead, nil
	}
	sstableRead, err := it.cacheManager.OpenSstCache.Get(ctx, it.sstFileNum,
		otter.LoaderFunc[uint64, *SSTableRead](it.sstReadFileLoader))

	if err != nil {
		return nil, err
	}
	if sstableRead.TryAcquire() {
		it.sstRead = sstableRead
		return it.sstRead, nil
	}

	return nil, fmt.Errorf("unable to pin sst file %d", it.sstFileNum)
}

func (it *SSTableIterator) sstBlockLoader(ctx context.Context, blockCacheKey BlockCacheKey) (*SSTBlock, error) {
	_, err := it.sstFile(ctx)
	if err != nil {
		return nil, err
	}

	return it.sstRead.LoadBlock(it.sstRead.SstFile, blockCacheKey.blockOffset, blockCacheKey.blockLen)
}

func (it *SSTableIterator) sstIndexLoader(ctx context.Context, sstFileNum uint64) (*SSTIndex, error) {
	_, err := it.sstFile(ctx)
	if err != nil {
		return nil, err
	}

	return it.sstRead.LoadIndex(it.sstRead.SstFile)
}

func (it *SSTableIterator) sstReadFileLoader(ctx context.Context, sstFileNum uint64) (*SSTableRead, error) {
	return OpenSSTableRead(sstFileNum)
}

func (it *SSTableIterator) loadSstableIndex() {
	sstIndex, err := it.cacheManager.SstIndexCache.Get(it.ctx, it.sstFileNum,
		otter.LoaderFunc[uint64, *SSTIndex](it.sstIndexLoader))

	if err != nil {
		fmt.Printf("unable to load sstable index %d", it.sstFileNum)
	}
	it.sstIndex = sstIndex
}
