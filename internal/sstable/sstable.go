package sstable

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc64"
	"math"
	"os"
	"path"
	"sort"
	"sync/atomic"

	"github.com/maypok86/otter/v2"
	"github.com/ssg2526/shunya/config"
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/memtable"
)

const (
	VERSION = uint16(1)
)

type SSTable struct {
	sstFile      *os.File
	bufWriter    *bufio.Writer
	headerOffset uint32
	footerOffset uint32
	dataOffset   uint32
	sstFileNum   uint64
}

type SSTableRead struct {
	SstFile       *os.File
	SstFileNum    uint64
	RefCounter    atomic.Int32
	blockCache    *otter.Cache[BlockCacheKey, SSTBlock]
	sstIndexCache *otter.Cache[uint64, SSTIndex]
	openSstCache  *otter.Cache[uint64, *SSTableRead]
}

type SSTBlock struct {
	BlockLen   uint32
	EntryCount uint32
	Entries    []SSTBlockEntry
	Checksum   uint64
}

type SSTBlockEntry struct {
	internalKey constants.InternalKey
	value       []byte
}

type SSTFooter struct {
	indexOffset uint32
	indexSize   uint32
	version     uint16
	magic       [8]byte
}

type SSTIndex struct {
	IndexEntries []SSTIndexEntry
	IndexSize    uint32
	Checksum     uint64
}

type SSTIndexEntry struct {
	BlockLen    uint32
	BlockOffset uint32
	Key         []byte            // composite key: user key of the block's first entry
	Lsn         constants.LsnType // lsn of the block's first entry, for the same reason
}

type FlushResult struct {
	MinKey []byte
	MaxKey []byte
	MinLsn constants.LsnType
	MaxLsn constants.LsnType
}

func OpenSSTable(fileNum uint64) *SSTable {
	sstFile, err := os.OpenFile(path.Join(config.ShunyaConfigs.DataDir, fmt.Sprintf("%016d", fileNum)+".sst"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("open new sstable file err", err)
	}
	sstable := &SSTable{
		sstFile:    sstFile,
		bufWriter:  bufio.NewWriterSize(sstFile, config.ShunyaConfigs.SSTWriteBufferSize),
		sstFileNum: fileNum,
	}

	return sstable
}

func OpenSSTableRead(fileNum uint64) (*SSTableRead, error) {
	sstFile, err := os.OpenFile(path.Join(config.ShunyaConfigs.DataDir, fmt.Sprintf("%016d", fileNum)+".sst"), os.O_RDONLY, 0444)
	if err != nil {
		fmt.Println("open new sstable file err", err)
		return &SSTableRead{}, err
	}

	sstableRead := &SSTableRead{
		SstFile:    sstFile,
		SstFileNum: fileNum,
	}
	sstableRead.RefCounter.Store(1)

	return sstableRead, nil
}

func (sstable *SSTable) CloseSSTable(fileNum uint64) {

}

func (sstable *SSTable) Flush(memtable memtable.Memtable) FlushResult {

	blockEntries := make([]SSTBlockEntry, 0)
	indexEntries := make([]SSTIndexEntry, 0)
	it := memtable.NewRawIterator()
	it.First()
	blockSize := 0
	minLsn := constants.LsnType(math.MaxInt64)
	maxLsn := constants.LsnType(0)
	maxKey := []byte(nil)
	offset := uint32(0)
	for it.Valid() {

		minLsn = min(it.Key().Lsn, minLsn)
		maxLsn = max(it.Key().Lsn, maxLsn)

		sstBlockEntry := SSTBlockEntry{
			internalKey: it.Key(),
			value:       it.Value(),
		}
		blockEntries = append(blockEntries, sstBlockEntry)
		blockSize += KEY_VAL_LEN_SIZE + len(it.Key().Key) + LSN_SIZE + ENTRY_TYPE_SIZE + len(it.Value())
		if blockSize >= SSTABLE_BLOCK_SIZE_BYTES {
			sstBlock := &SSTBlock{
				EntryCount: uint32(len(blockEntries)),
				Entries:    blockEntries,
			}
			marshalledData := MarshalSSTDataBlock(sstBlock)
			sstable.bufWriter.Write(marshalledData)

			indexEntry := SSTIndexEntry{
				BlockLen:    sstBlock.BlockLen,
				BlockOffset: offset,
				Key:         sstBlock.Entries[0].internalKey.Key,
				Lsn:         sstBlock.Entries[0].internalKey.Lsn,
			}
			offset += sstBlock.BlockLen
			indexEntries = append(indexEntries, indexEntry)

			blockSize = 0
			blockEntries = make([]SSTBlockEntry, 0)
		}
		maxKey = it.Key().Key

		it.Next()
	}
	if blockSize > 0 {
		sstBlock := &SSTBlock{
			EntryCount: uint32(len(blockEntries)),
			Entries:    blockEntries,
		}
		marshalledData := MarshalSSTDataBlock(sstBlock)
		sstable.bufWriter.Write(marshalledData)

		indexEntry := SSTIndexEntry{
			BlockLen:    sstBlock.BlockLen,
			BlockOffset: offset,
			Key:         sstBlock.Entries[0].internalKey.Key,
			Lsn:         sstBlock.Entries[0].internalKey.Lsn,
		}
		offset += sstBlock.BlockLen
		indexEntries = append(indexEntries, indexEntry)
	}
	sstIndex := &SSTIndex{
		IndexEntries: indexEntries,
	}
	marshalledIndexData := MarshalSSTIndex(sstIndex)
	sstable.bufWriter.Write(marshalledIndexData)

	sstFooter := &SSTFooter{
		indexOffset: offset,
		indexSize:   sstIndex.IndexSize,
		version:     VERSION,
	}
	marshalledFooterData := MarshalSSTFooter(sstFooter)
	sstable.bufWriter.Write(marshalledFooterData)
	sstable.bufWriter.Flush()

	return FlushResult{
		MinKey: sstIndex.IndexEntries[0].Key,
		MaxKey: maxKey,
		MinLsn: minLsn,
		MaxLsn: maxLsn,
	}

}

func compareInternalKey(key []byte, lsn constants.LsnType, targetKey []byte, targetLsn constants.LsnType) int {
	if c := bytes.Compare(key, targetKey); c != 0 {
		return c
	}
	return cmp.Compare(targetLsn, lsn)
}

func (sstIndex *SSTIndex) FindBlock(key []byte, snapshotLsn constants.LsnType) (int32, bool) {
	entries := sstIndex.IndexEntries
	i := sort.Search(len(entries), func(i int) bool {
		return compareInternalKey(entries[i].Key, entries[i].Lsn, key, snapshotLsn) > 0
	})

	if i == 0 {
		if !bytes.Equal(entries[0].Key, key) {
			return -1, false
		}
		return 0, true
	}

	return int32(i - 1), true
}

func (sstIndex *SSTIndex) FindRangeStartBlock(startKey []byte, snapshotLsn constants.LsnType) (int32, bool) {
	entries := sstIndex.IndexEntries
	i := sort.Search(len(entries), func(i int) bool {
		return compareInternalKey(entries[i].Key, entries[i].Lsn, startKey, snapshotLsn) > 0
	})

	if i == 0 {
		return 0, true
	}

	return int32(i - 1), true
}

func (sstBlock *SSTBlock) Search(key []byte, snapshotLsn constants.LsnType) (int32, bool) {
	entries := sstBlock.Entries

	i := sort.Search(len(entries), func(i int) bool {
		internalKey := entries[i].internalKey
		return compareInternalKey(internalKey.Key, internalKey.Lsn, key, snapshotLsn) >= 0
	})

	if i < len(entries) {
		candidateEntry := entries[i].internalKey

		if bytes.Equal(candidateEntry.Key, key) {
			return int32(i), true
		}
	}

	return -1, false
}

func (sstBlock *SSTBlock) SearchForRange(key []byte, snapshotLsn constants.LsnType) (int32, bool) {
	entries := sstBlock.Entries

	i := sort.Search(len(entries), func(i int) bool {
		internalKey := entries[i].internalKey
		return compareInternalKey(internalKey.Key, internalKey.Lsn, key, snapshotLsn) >= 0
	})

	return int32(i), i < len(entries)
}

func (sstableRead *SSTableRead) LoadBlock(sstFile *os.File, blockOffset uint32, blockLen uint32) (*SSTBlock, error) {
	if blockLen < BLOCK_TRAILER_SIZE {
		return nil, fmt.Errorf("sst %d: block at offset %d is too small (%d bytes)", sstableRead.SstFileNum, blockOffset, blockLen)
	}

	blockBytes := make([]byte, blockLen)
	if _, err := sstFile.ReadAt(blockBytes, int64(blockOffset)); err != nil {
		return nil, err
	}

	checksumStart := blockLen - CHECKSUM_SIZE
	storedChecksum := binary.LittleEndian.Uint64(blockBytes[checksumStart:])
	if crc64.Checksum(blockBytes[:checksumStart], crc64Table) != storedChecksum {
		return nil, fmt.Errorf("sst %d: block at offset %d checksum mismatch", sstableRead.SstFileNum, blockOffset)
	}

	return UnMarshalSSTableBlock(blockBytes), nil
}

func (sstableRead *SSTableRead) LoadIndex(sstFile *os.File) (*SSTIndex, error) {
	sstFooter, err := sstableRead.readFooter(sstFile)
	if err != nil {
		//log
		return nil, err
	}
	return sstableRead.readIndex(sstFile, &sstFooter)
}

func (sstableRead *SSTableRead) readFooter(sstFile *os.File) (SSTFooter, error) {
	stat, err := sstFile.Stat()
	if err != nil {
		//log err
		return SSTFooter{}, err
	}
	footerBytes := make([]byte, FOOTER_SIZE_BYTES)
	if _, err := sstFile.ReadAt(footerBytes, stat.Size()-FOOTER_SIZE_BYTES); err != nil {
		return SSTFooter{}, err
	}

	footerByterOffset := 0
	indexOffset := binary.LittleEndian.Uint32(footerBytes[footerByterOffset:FOOTER_INDEX_OFFSET_SIZE])
	footerByterOffset += FOOTER_INDEX_OFFSET_SIZE
	indexLen := binary.LittleEndian.Uint32(footerBytes[footerByterOffset : footerByterOffset+FOOTER_INDEX_LEN_SIZE])
	footerByterOffset += FOOTER_INDEX_LEN_SIZE
	version := binary.LittleEndian.Uint16(footerBytes[footerByterOffset : footerByterOffset+FOOTER_VERSION_SIZE])
	footerByterOffset += FOOTER_VERSION_SIZE
	magic := footerBytes[footerByterOffset : footerByterOffset+FOOTER_MAGIC_SIZE]

	if len(magic) != 8 { //add magic num check as well
		return SSTFooter{}, errors.New("magic number mismatch file corrupted")
	}

	return SSTFooter{
		indexOffset: indexOffset,
		indexSize:   indexLen,
		version:     version,
		magic:       [8]byte(magic),
	}, nil
}

func (sstableRead *SSTableRead) readIndex(sstFile *os.File, sstFooter *SSTFooter) (*SSTIndex, error) {
	indexBytes := make([]byte, sstFooter.indexSize)
	if _, err := sstFile.ReadAt(indexBytes, int64(sstFooter.indexOffset)); err != nil {
		//log
		return nil, err
	}
	sstIndex := UnMarshalSSTIndex(indexBytes)

	return sstIndex, nil
}

func (sstableRead *SSTableRead) TryAcquire() bool {
	for {
		curRefCounter := sstableRead.RefCounter.Load()
		if curRefCounter <= 0 {
			return false
		}
		if sstableRead.RefCounter.CompareAndSwap(curRefCounter, curRefCounter+1) {
			return true
		}
	}
}

func (sstableRead *SSTableRead) Release() {
	if sstableRead.RefCounter.Add(-1) == 0 {
		sstableRead.CloseSST()
	}
}

func onSstOpenFileCacheEvict(ssTableRead *SSTableRead) {
	ssTableRead.Release()
}

func (sstableRead *SSTableRead) CloseSST() {
	err := sstableRead.SstFile.Close()
	if err != nil {
		fmt.Println("error closing sst file for sstableRead: " + sstableRead.SstFile.Name())
	}
}
