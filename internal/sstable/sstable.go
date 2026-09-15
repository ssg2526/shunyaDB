package sstable

import (
	"bufio"
	"fmt"
	"os"
	"path"

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
}

type SSTBlock struct {
	blockLen   uint32
	entryCount uint32
	entries    []SSTBlockEntry
	checksum   uint64
}

type SSTBlockEntry struct {
	internalKey SSTableInternalKey
	value       []byte
}

type SSTableInternalKey struct {
	key       []byte
	lsn       constants.LsnType
	entryType constants.EntryType
}

type SSTFooter struct {
	indexOffset uint32
	indexLen    uint32
	version     uint16
	magic       [8]byte
}

type SSTIndex struct {
	indexEntries []SSTIndexEntry
	indexLen     uint32
	checksum     uint64
}

type SSTIndexEntry struct {
	blockLen    uint32
	blockOffset uint32
	key         []byte            // composite key: user key of the block's first entry
	lsn         constants.LsnType // lsn of the block's first entry, for the same reason
}

// TODO: use real file name
func OpenSSTable() *SSTable {
	sstFile, err := os.OpenFile(path.Join(config.ShunyaConfigs.SSTableDir, "sstable1"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("open new sstable file err", err)
	}
	sstable := &SSTable{
		sstFile: sstFile,
	}
	sstable.bufWriter = bufio.NewWriterSize(sstFile, config.ShunyaConfigs.SSTWriteBufferSize)

	return sstable
}

func (sstable *SSTable) Flush(memtable memtable.Memtable) {

	blockEntries := make([]SSTBlockEntry, 0)
	indexEntries := make([]SSTIndexEntry, 0)
	it := memtable.NewVersionedIterator()
	blockSize := 0
	offset := uint32(0)
	for it.Valid() {
		versions := it.Versions()
		for _, version := range versions {
			sstBlockEntry := SSTBlockEntry{
				internalKey: SSTableInternalKey{key: it.Key(), lsn: version.Lsn, entryType: version.EntryType},
				value:       version.Value,
			}
			blockEntries = append(blockEntries, sstBlockEntry)
			blockSize += KEY_VAL_LEN_SIZE + len(it.Key()) + LSN_SIZE + ENTRY_TYPE_SIZE + len(version.Value)
			if blockSize >= SSTABLE_BLOCK_SIZE_BYTES {
				sstBlock := &SSTBlock{
					entryCount: uint32(len(blockEntries)),
					entries:    blockEntries,
				}
				marshalledData := MarshalSSTDataBlock(sstBlock)
				sstable.bufWriter.Write(marshalledData)

				indexEntry := SSTIndexEntry{
					blockLen:    sstBlock.blockLen,
					blockOffset: offset,
					key:         sstBlock.entries[0].internalKey.key,
					lsn:         sstBlock.entries[0].internalKey.lsn,
				}
				offset += sstBlock.blockLen
				indexEntries = append(indexEntries, indexEntry)

				blockSize = 0
				blockEntries = make([]SSTBlockEntry, 0)
			}
		}
		it.Next()
	}
	if blockSize > 0 {
		sstBlock := &SSTBlock{
			entryCount: uint32(len(blockEntries)),
			entries:    blockEntries,
		}
		marshalledData := MarshalSSTDataBlock(sstBlock)
		sstable.bufWriter.Write(marshalledData)

		indexEntry := SSTIndexEntry{
			blockLen:    sstBlock.blockLen,
			blockOffset: offset,
			key:         sstBlock.entries[0].internalKey.key,
			lsn:         sstBlock.entries[0].internalKey.lsn,
		}
		offset += sstBlock.blockLen
		indexEntries = append(indexEntries, indexEntry)
	}
	sstIndex := &SSTIndex{
		indexEntries: indexEntries,
	}
	marshalledIndexData := MarshalSSTIndex(sstIndex)
	sstable.bufWriter.Write(marshalledIndexData)

	sstFooter := &SSTFooter{
		indexOffset: offset,
		indexLen:    sstIndex.indexLen,
		version:     VERSION,
	}
	marshalledFooterData := MarshalSSTFooter(sstFooter)
	sstable.bufWriter.Write(marshalledFooterData)
	sstable.bufWriter.Flush()
}

func (sstable *SSTable) ReadFooter() {

}

func (sstable *SSTable) ReadIndex() {

}
