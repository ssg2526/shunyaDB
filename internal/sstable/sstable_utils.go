package sstable

import (
	"encoding/binary"
	"hash/crc64"

	constants "github.com/ssg2526/shunya/internal/constants"
)

var crc64Table = crc64.MakeTable(crc64.ISO)

const (
	SSTABLE_BLOCK_SIZE_BYTES = 4096
	LSN_SIZE                 = 8
	KEY_LEN_SIZE             = 8
	VAL_LEN_SIZE             = 8
	ENTRY_TYPE_SIZE          = 1
	KEY_VAL_LEN_SIZE         = KEY_LEN_SIZE + VAL_LEN_SIZE
	ENTRY_COUNT_SIZE         = 4
	CHECKSUM_SIZE            = 8
	BLOCK_TRAILER_SIZE       = ENTRY_COUNT_SIZE + CHECKSUM_SIZE
	FOOTER_SIZE_BYTES        = 18

	INDEX_BLOCK_LEN_SIZE    = 4
	INDEX_BLOCK_OFFSET_SIZE = 4

	FOOTER_INDEX_OFFSET_SIZE = 4
	FOOTER_INDEX_LEN_SIZE    = 4
	FOOTER_VERSION_SIZE      = 2
	FOOTER_MAGIC_SIZE        = 8
)

func MarshalSSTDataBlock(sstableBlock *SSTBlock) []byte {
	dataSize := 0
	for _, entry := range sstableBlock.Entries {
		dataSize += KEY_VAL_LEN_SIZE + len(entry.internalKey.key) + LSN_SIZE + ENTRY_TYPE_SIZE + len(entry.value)
	}

	entryCount := uint32(len(sstableBlock.Entries))
	totalSize := dataSize + BLOCK_TRAILER_SIZE

	marshalled := make([]byte, totalSize)

	offset := 0
	for _, entry := range sstableBlock.Entries {
		key := entry.internalKey.key
		value := entry.value

		binary.LittleEndian.PutUint64(marshalled[offset:], uint64(len(key)))
		offset += KEY_LEN_SIZE

		binary.LittleEndian.PutUint64(marshalled[offset:], uint64(len(value)))
		offset += VAL_LEN_SIZE

		copy(marshalled[offset:], key)
		offset += len(key)

		binary.LittleEndian.PutUint64(marshalled[offset:], uint64(entry.internalKey.lsn))
		offset += LSN_SIZE

		marshalled[offset] = byte(entry.internalKey.entryType)
		offset += ENTRY_TYPE_SIZE

		copy(marshalled[offset:], value)
		offset += len(value)
	}

	binary.LittleEndian.PutUint32(marshalled[offset:], entryCount)
	offset += ENTRY_COUNT_SIZE

	checksum := crc64.Checksum(marshalled[:offset], crc64Table)
	binary.LittleEndian.PutUint64(marshalled[offset:], checksum)

	sstableBlock.BlockLen = uint32(totalSize)
	sstableBlock.EntryCount = entryCount
	sstableBlock.Checksum = checksum

	return marshalled
}

func UnMarshalSSTableBlock(sstableBlockBytes []byte) *SSTBlock {
	blockLen := len(sstableBlockBytes)

	trailerStart := blockLen - BLOCK_TRAILER_SIZE
	sstBlockEntryCount := binary.LittleEndian.Uint32(sstableBlockBytes[trailerStart : trailerStart+ENTRY_COUNT_SIZE])
	sstBlockChecksum := binary.LittleEndian.Uint64(sstableBlockBytes[trailerStart+ENTRY_COUNT_SIZE:])

	sstBlockEntries := make([]SSTBlockEntry, 0, sstBlockEntryCount)

	offset := uint64(0)
	for i := uint32(0); i < sstBlockEntryCount; i++ {

		keyLen := binary.LittleEndian.Uint64(sstableBlockBytes[offset : offset+KEY_LEN_SIZE])
		valLen := binary.LittleEndian.Uint64(sstableBlockBytes[offset+KEY_LEN_SIZE : offset+KEY_VAL_LEN_SIZE])

		keyStart := offset + KEY_VAL_LEN_SIZE
		key := sstableBlockBytes[keyStart : keyStart+keyLen]

		lsnStart := keyStart + keyLen
		lsn := binary.LittleEndian.Uint64(sstableBlockBytes[lsnStart : lsnStart+LSN_SIZE])

		entryTypeStart := lsnStart + LSN_SIZE
		entryType := sstableBlockBytes[entryTypeStart]

		valStart := entryTypeStart + ENTRY_TYPE_SIZE
		value := sstableBlockBytes[valStart : valStart+valLen]

		sstBlockEntries = append(sstBlockEntries, SSTBlockEntry{
			internalKey: SSTableInternalKey{
				key:       key,
				lsn:       constants.LsnType(lsn),
				entryType: constants.EntryType(entryType),
			},
			value: value,
		})

		offset = valStart + valLen
	}

	return &SSTBlock{
		BlockLen:   uint32(blockLen),
		EntryCount: sstBlockEntryCount,
		Entries:    sstBlockEntries,
		Checksum:   sstBlockChecksum,
	}
}

func MarshalSSTIndex(sstIndex *SSTIndex) []byte {
	dataSize := 0
	for _, entry := range sstIndex.IndexEntries {
		dataSize += INDEX_BLOCK_LEN_SIZE + INDEX_BLOCK_OFFSET_SIZE + KEY_LEN_SIZE + len(entry.Key) + LSN_SIZE
	}

	entryCount := uint32(len(sstIndex.IndexEntries))
	totalSize := dataSize + BLOCK_TRAILER_SIZE

	marshalled := make([]byte, totalSize)

	offset := 0
	for _, entry := range sstIndex.IndexEntries {
		binary.LittleEndian.PutUint32(marshalled[offset:], entry.BlockLen)
		offset += INDEX_BLOCK_LEN_SIZE

		binary.LittleEndian.PutUint32(marshalled[offset:], entry.BlockOffset)
		offset += INDEX_BLOCK_OFFSET_SIZE

		binary.LittleEndian.PutUint64(marshalled[offset:], uint64(len(entry.Key)))
		offset += KEY_LEN_SIZE

		copy(marshalled[offset:], entry.Key)
		offset += len(entry.Key)

		binary.LittleEndian.PutUint64(marshalled[offset:], uint64(entry.Lsn))
		offset += LSN_SIZE
	}

	binary.LittleEndian.PutUint32(marshalled[offset:], entryCount)
	offset += ENTRY_COUNT_SIZE

	checksum := crc64.Checksum(marshalled[:offset], crc64Table)
	binary.LittleEndian.PutUint64(marshalled[offset:], checksum)

	sstIndex.IndexSize = uint32(totalSize)
	sstIndex.Checksum = checksum

	return marshalled
}

func UnMarshalSSTIndex(sstIndexBytes []byte) *SSTIndex {
	indexSize := len(sstIndexBytes)

	trailerStart := indexSize - BLOCK_TRAILER_SIZE
	entryCount := binary.LittleEndian.Uint32(sstIndexBytes[trailerStart : trailerStart+ENTRY_COUNT_SIZE])
	checksum := binary.LittleEndian.Uint64(sstIndexBytes[trailerStart+ENTRY_COUNT_SIZE:])

	indexEntries := make([]SSTIndexEntry, 0, entryCount)

	offset := uint64(0)
	for i := uint32(0); i < entryCount; i++ {
		blockLen := binary.LittleEndian.Uint32(sstIndexBytes[offset : offset+INDEX_BLOCK_LEN_SIZE])
		blockOffset := binary.LittleEndian.Uint32(sstIndexBytes[offset+INDEX_BLOCK_LEN_SIZE : offset+INDEX_BLOCK_LEN_SIZE+INDEX_BLOCK_OFFSET_SIZE])

		keyLenStart := offset + INDEX_BLOCK_LEN_SIZE + INDEX_BLOCK_OFFSET_SIZE
		keyLen := binary.LittleEndian.Uint64(sstIndexBytes[keyLenStart : keyLenStart+KEY_LEN_SIZE])

		keyStart := keyLenStart + KEY_LEN_SIZE
		key := sstIndexBytes[keyStart : keyStart+keyLen]

		lsnStart := keyStart + keyLen
		lsn := binary.LittleEndian.Uint64(sstIndexBytes[lsnStart : lsnStart+LSN_SIZE])

		indexEntries = append(indexEntries, SSTIndexEntry{
			BlockLen:    blockLen,
			BlockOffset: blockOffset,
			Key:         key,
			Lsn:         constants.LsnType(lsn),
		})

		offset = lsnStart + LSN_SIZE
	}

	return &SSTIndex{
		IndexEntries: indexEntries,
		IndexSize:    uint32(indexSize),
		Checksum:     checksum,
	}
}

func MarshalSSTFooter(sstFooter *SSTFooter) []byte {
	marshalled := make([]byte, FOOTER_SIZE_BYTES)

	offset := 0
	binary.LittleEndian.PutUint32(marshalled[offset:offset+FOOTER_INDEX_OFFSET_SIZE], sstFooter.indexOffset)
	offset += FOOTER_INDEX_OFFSET_SIZE

	binary.LittleEndian.PutUint32(marshalled[offset:offset+FOOTER_INDEX_LEN_SIZE], sstFooter.indexSize)
	offset += FOOTER_INDEX_LEN_SIZE

	binary.LittleEndian.PutUint16(marshalled[offset:offset+FOOTER_VERSION_SIZE], sstFooter.version)
	offset += FOOTER_VERSION_SIZE

	copy(marshalled[offset:], sstFooter.magic[:])

	return marshalled
}

func UnMarshalSSTFooter(sstFooterBytes []byte) *SSTFooter {
	offset := 0

	indexOffset := binary.LittleEndian.Uint32(sstFooterBytes[offset:])
	offset += FOOTER_INDEX_OFFSET_SIZE

	indexLen := binary.LittleEndian.Uint32(sstFooterBytes[offset:])
	offset += FOOTER_INDEX_LEN_SIZE

	version := binary.LittleEndian.Uint16(sstFooterBytes[offset:])
	offset += FOOTER_VERSION_SIZE

	var magic [8]byte
	copy(magic[:], sstFooterBytes[offset:])

	return &SSTFooter{
		indexOffset: indexOffset,
		indexSize:   indexLen,
		version:     version,
		magic:       magic,
	}
}
