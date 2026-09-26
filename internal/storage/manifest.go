package storage

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc64"
	"os"
	"path/filepath"

	constants "github.com/ssg2526/shunya/internal/constants"

	"github.com/ssg2526/shunya/config"
)

const (
	MANIFEST_FILE_SUFFIX = "MANIFEST"
	CURRENT_POINTER_FILE = "CURRENT"

	MANIFEST_OP_COUNT_SIZE       = 2
	MANIFEST_ENTRY_BYTES_SIZE    = 4
	MANIFEST_KEY_LEN_SIZE        = 4
	MANIFEST_FILE_NUM_SIZE       = 8
	MANIFEST_ENTRY_TYPE_SIZE     = 1
	MANIFEST_LEVEL_SIZE          = 4
	MANIFEST_LSN_SIZE            = 8
	MANIFEST_CHECKSUM_SIZE       = 8
	MANIFEST_RECORD_SIZE_WO_KEYS = MANIFEST_FILE_NUM_SIZE + MANIFEST_ENTRY_TYPE_SIZE + MANIFEST_LEVEL_SIZE + 2*MANIFEST_LSN_SIZE + 2*MANIFEST_KEY_LEN_SIZE
)

var manifestCrc64Table = crc64.MakeTable(crc64.ISO)

type Manifest struct {
	manifestFile *os.File
	bufWriter    *bufio.Writer
	fileNum      uint64
	size         int
}

type ManifestOp struct {
	minKey     []byte
	maxKey     []byte
	sstFileNum uint64
	entryType  constants.EntryType
	level      uint32
	minLsn     constants.LsnType
	maxLsn     constants.LsnType
}

type SSTableMeta struct {
}

type ManifestState struct {
	SSTByMeta [][]SSTableMeta
	MaxLsn    constants.LsnType
}

type ManifestEntry struct {
	manifestOps []ManifestOp
}

func InitManifest() (manifest *Manifest) {

	manifestFile, fileNum, size := GetCurrentManifest(config.ShunyaConfigs.ManifestDir)
	bufWriter := bufio.NewWriterSize(manifestFile, config.ShunyaConfigs.ManifestWriteBufferSize)

	return &Manifest{
		manifestFile: manifestFile,
		bufWriter:    bufWriter,
		fileNum:      fileNum,
		size:         size,
	}
}

func (manifest *Manifest) MarshalManifestEntry(manifestEntry *ManifestEntry) []byte {

	totalKeyLengths := 0
	for _, manifestOp := range manifestEntry.manifestOps {
		totalKeyLengths += len(manifestOp.minKey) + len(manifestOp.maxKey)
	}
	opCount := len(manifestEntry.manifestOps)
	opsSize := opCount*MANIFEST_RECORD_SIZE_WO_KEYS + totalKeyLengths
	totalSize := MANIFEST_OP_COUNT_SIZE + MANIFEST_ENTRY_BYTES_SIZE + opsSize + MANIFEST_CHECKSUM_SIZE

	marshalled := make([]byte, totalSize)

	offset := 0
	binary.LittleEndian.PutUint16(marshalled[offset:], uint16(opCount))
	offset += MANIFEST_OP_COUNT_SIZE

	binary.LittleEndian.PutUint32(marshalled[offset:], uint32(opsSize))
	offset += MANIFEST_ENTRY_BYTES_SIZE

	for _, manifestOp := range manifestEntry.manifestOps {
		minKeyLen := len(manifestOp.minKey)
		maxKeyLen := len(manifestOp.maxKey)

		binary.LittleEndian.PutUint32(marshalled[offset:], uint32(minKeyLen))
		offset += MANIFEST_KEY_LEN_SIZE
		copy(marshalled[offset:], manifestOp.minKey)
		offset += minKeyLen

		binary.LittleEndian.PutUint32(marshalled[offset:], uint32(maxKeyLen))
		offset += MANIFEST_KEY_LEN_SIZE
		copy(marshalled[offset:], manifestOp.maxKey)
		offset += maxKeyLen

		binary.LittleEndian.PutUint64(marshalled[offset:], manifestOp.sstFileNum)
		offset += MANIFEST_FILE_NUM_SIZE

		marshalled[offset] = byte(manifestOp.entryType)
		offset += MANIFEST_ENTRY_TYPE_SIZE

		binary.LittleEndian.PutUint32(marshalled[offset:], manifestOp.level)
		offset += MANIFEST_LEVEL_SIZE

		binary.LittleEndian.PutUint64(marshalled[offset:], uint64(manifestOp.minLsn))
		offset += MANIFEST_LSN_SIZE

		binary.LittleEndian.PutUint64(marshalled[offset:], uint64(manifestOp.maxLsn))
		offset += MANIFEST_LSN_SIZE
	}

	checksum := crc64.Checksum(marshalled[:offset], manifestCrc64Table)
	binary.LittleEndian.PutUint64(marshalled[offset:], checksum)

	return marshalled
}

func (manifest *Manifest) UnMarshalManifestEntry(marshalEntryBytes []byte) (manifestEntry *ManifestEntry) {
	offset := 0

	opCount := binary.LittleEndian.Uint16(marshalEntryBytes[offset:])
	offset += MANIFEST_OP_COUNT_SIZE

	// opsSize is only needed by the caller/replay logic for a pre-flight
	// "do I have enough bytes before EOF" check; not needed to decode here.
	offset += MANIFEST_ENTRY_BYTES_SIZE

	manifestOps := make([]ManifestOp, 0, opCount)

	for i := uint16(0); i < opCount; i++ {
		minKeyLen := binary.LittleEndian.Uint32(marshalEntryBytes[offset:])
		offset += MANIFEST_KEY_LEN_SIZE
		minKey := marshalEntryBytes[offset : offset+int(minKeyLen)]
		offset += int(minKeyLen)

		maxKeyLen := binary.LittleEndian.Uint32(marshalEntryBytes[offset:])
		offset += MANIFEST_KEY_LEN_SIZE
		maxKey := marshalEntryBytes[offset : offset+int(maxKeyLen)]
		offset += int(maxKeyLen)

		sstFileNum := binary.LittleEndian.Uint64(marshalEntryBytes[offset:])
		offset += MANIFEST_FILE_NUM_SIZE

		entryType := constants.EntryType(marshalEntryBytes[offset])
		offset += MANIFEST_ENTRY_TYPE_SIZE

		level := binary.LittleEndian.Uint32(marshalEntryBytes[offset:])
		offset += MANIFEST_LEVEL_SIZE

		minLsn := constants.LsnType(binary.LittleEndian.Uint64(marshalEntryBytes[offset:]))
		offset += MANIFEST_LSN_SIZE

		maxLsn := constants.LsnType(binary.LittleEndian.Uint64(marshalEntryBytes[offset:]))
		offset += MANIFEST_LSN_SIZE

		manifestOps = append(manifestOps, ManifestOp{
			minKey:     minKey,
			maxKey:     maxKey,
			sstFileNum: sstFileNum,
			entryType:  entryType,
			level:      level,
			minLsn:     minLsn,
			maxLsn:     maxLsn,
		})
	}
	// remaining bytes at marshalEntryBytes[offset:] are the trailing checksum,
	// left to the caller/replay logic to verify since ManifestEntry doesn't carry it.

	return &ManifestEntry{
		manifestOps: manifestOps,
	}
}

func (manifest *Manifest) AppendToManifest(manifestEntry *ManifestEntry) error {
	//TODO: handle concurrency
	manifestEntryBytes := manifest.MarshalManifestEntry(manifestEntry)
	if _, err := manifest.bufWriter.Write(manifestEntryBytes); err != nil {
		return err
	}
	manifest.bufWriter.Flush()
	manifest.manifestFile.Sync()

	return nil
}

func (manifest *Manifest) ReplayManifestFile() (manifestState ManifestState) {
	GetCurrentManifest(config.ShunyaConfigs.ManifestDir)
	return ManifestState{}
}

func GetCurrentManifest(manifestDir string) (*os.File, uint64, int) {

	if _, err := os.Stat(filepath.Join(manifestDir, CURRENT_POINTER_FILE)); errors.Is(err, os.ErrNotExist) {
		fileNum := uint64(1)
		manFilename := fmt.Sprintf("%016d", fileNum) + "_" + MANIFEST_FILE_SUFFIX
		manifestFile, err := os.OpenFile(filepath.Join(manifestDir, manFilename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("open new manifest file err", err)
		}
		fileNumBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(fileNumBytes, fileNum)
		currfile_err := os.WriteFile(filepath.Join(manifestDir, CURRENT_POINTER_FILE), fileNumBytes, 0644)
		if currfile_err != nil {
			fmt.Println("open new man curret pointer file err", currfile_err)
		}
		return manifestFile, fileNum, 0
	}
	manifestNum, err := GetCurrentManifestNumFromCurrent(manifestDir)
	manFileName := fmt.Sprintf("%016d", manifestNum) + "_" + MANIFEST_FILE_SUFFIX
	manifestFile, err := os.OpenFile(filepath.Join(manifestDir, manFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("open new manifest file err", err)
	}
	info, _ := manifestFile.Stat()
	return manifestFile, manifestNum, int(info.Size())

}

func GetCurrentManifestNumFromCurrent(manifestDir string) (uint64, error) {
	currPtrFilename := filepath.Join(manifestDir, CURRENT_POINTER_FILE)

	if _, err := os.Stat(currPtrFilename); errors.Is(err, os.ErrNotExist) {
		fmt.Println("Curr Pointer File does not exist")
		return 0, err
	} else if err != nil {
		fmt.Printf("Error checking file: %v\n", err)
		return 0, err
	}
	manFileNumStr, err := os.ReadFile(currPtrFilename)
	if err != nil {
		fmt.Println("Curr Pointer File read error", err)
	}

	return binary.LittleEndian.Uint64(manFileNumStr), nil
}

func updateCurrentPtrFileAtomically(dataDir string, currentManifestFileNum uint64) error {
	tmpPtrFilename := filepath.Join(dataDir, CURRENT_POINTER_FILE+".tmp")
	currPtrFileName := filepath.Join(dataDir, CURRENT_POINTER_FILE)

	tmpPtrFile, err := os.OpenFile(tmpPtrFilename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	fileNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(fileNumBytes, currentManifestFileNum)
	if _, err := tmpPtrFile.Write(fileNumBytes); err != nil {
		tmpPtrFile.Close()
		return err
	}

	if err := tmpPtrFile.Sync(); err != nil {
		tmpPtrFile.Close()
		return err
	}

	if err := tmpPtrFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPtrFilename, currPtrFileName)
}
