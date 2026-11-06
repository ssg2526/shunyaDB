package sstable

import (
	"github.com/maypok86/otter/v2"
	"github.com/ssg2526/shunya/config"
)

type CacheManager struct {
	BlockCache    *otter.Cache[BlockCacheKey, *SSTBlock]
	SstIndexCache *otter.Cache[uint64, *SSTIndex]
	OpenSstCache  *otter.Cache[uint64, *SSTableRead]
}

type BlockCacheKey struct {
	sstFileNum  uint64
	blockOffset uint32
	blockLen    uint32
}

func InitCacheManager() *CacheManager {
	openSstCache, err := otter.New(&otter.Options[uint64, *SSTableRead]{
		MaximumSize: config.ShunyaConfigs.OpenSstLRUSize,
		OnDeletion: func(e otter.DeletionEvent[uint64, *SSTableRead]) {
			onSstOpenFileCacheEvict(e.Value)
		},
	})

	if err != nil {
		// log error
	}

	sstIndexCache, err := otter.New(&otter.Options[uint64, *SSTIndex]{
		MaximumWeight: uint64(config.ShunyaConfigs.SSTIndexCacheSizeByte),
		Weigher: func(key uint64, value *SSTIndex) uint32 {
			return value.IndexSize
		},
	})
	if err != nil {
		// log error
	}

	blockCache, err := otter.New(&otter.Options[BlockCacheKey, *SSTBlock]{
		MaximumWeight: uint64(config.ShunyaConfigs.BlockCacheSizeByte),
		Weigher: func(key BlockCacheKey, value *SSTBlock) uint32 {
			return value.BlockLen
		},
	})

	if err != nil {
		//log error
	}

	return &CacheManager{
		OpenSstCache:  openSstCache,
		SstIndexCache: sstIndexCache,
		BlockCache:    blockCache,
	}
}
