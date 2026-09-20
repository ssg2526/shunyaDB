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

	MANIFEST_ENTRY_COUNT_SIZE = 4
	MANIFEST_FILE_NUM_SIZE    = 8
	MANIFEST_ENTRY_TYPE_SIZE  = 1
	MANIFEST_CHECKSUM_SIZE    = 8
	MANIFEST_RECORD_SIZE      = MANIFEST_FILE_NUM_SIZE + MANIFEST_ENTRY_TYPE_SIZE
)

var manifestCrc64Table = crc64.MakeTable(crc64.ISO)

type Manifest struct {
	manifestFile *os.File
	bufWriter    *bufio.Writer
	fileNum      uint64
	size         int
}

type ManifestEntry struct {
	sstFileNum []uint64
	entryType  []constants.EntryType
}

func InitManifest() (manifest *Manifest) {

	manifestFile, fileNum, size := GetCurrentManifest(config.ShunyaConfigs.DataDir)
	bufWriter := bufio.NewWriterSize(manifestFile, config.ShunyaConfigs.ManifestWriteBufferSize)

	return &Manifest{
		manifestFile: manifestFile,
		bufWriter:    bufWriter,
		fileNum:      fileNum,
		size:         size,
	}
}

func (manifest *Manifest) MarshalManifestEntry(manifestEntry *ManifestEntry) []byte {
	entryCount := uint32(len(manifestEntry.sstFileNum))
	totalSize := MANIFEST_ENTRY_COUNT_SIZE + int(entryCount)*MANIFEST_RECORD_SIZE + MANIFEST_CHECKSUM_SIZE

	marshalled := make([]byte, totalSize)

	offset := 0
	binary.LittleEndian.PutUint32(marshalled[offset:], entryCount)
	offset += MANIFEST_ENTRY_COUNT_SIZE

	for i := 0; i < int(entryCount); i++ {
		binary.LittleEndian.PutUint64(marshalled[offset:], manifestEntry.sstFileNum[i])
		offset += MANIFEST_FILE_NUM_SIZE

		marshalled[offset] = byte(manifestEntry.entryType[i])
		offset += MANIFEST_ENTRY_TYPE_SIZE
	}

	checksum := crc64.Checksum(marshalled[:offset], manifestCrc64Table)
	binary.LittleEndian.PutUint64(marshalled[offset:], checksum)

	return marshalled
}

func (manifest *Manifest) UnMarshalManifestEntry(marshalEntryBytes []byte) (manifestEntry *ManifestEntry) {
	offset := 0
	entryCount := binary.LittleEndian.Uint32(marshalEntryBytes[offset:])
	offset += MANIFEST_ENTRY_COUNT_SIZE

	sstFileNum := make([]uint64, entryCount)
	entryType := make([]constants.EntryType, entryCount)

	for i := uint32(0); i < entryCount; i++ {
		sstFileNum[i] = binary.LittleEndian.Uint64(marshalEntryBytes[offset:])
		offset += MANIFEST_FILE_NUM_SIZE

		entryType[i] = constants.EntryType(marshalEntryBytes[offset])
		offset += MANIFEST_ENTRY_TYPE_SIZE
	}
	// remaining bytes at marshalEntryBytes[offset:] are the trailing checksum,
	// left to the caller/replay logic to verify since ManifestEntry doesn't carry it.

	return &ManifestEntry{
		sstFileNum: sstFileNum,
		entryType:  entryType,
	}
}

func (manifest *Manifest) AppendToManifest(manifestEntry *ManifestEntry) bool {
	manifestEntryBytes := manifest.MarshalManifestEntry(manifestEntry)
	if _, err := manifest.bufWriter.Write(manifestEntryBytes); err != nil {
		return false
	}
	manifest.bufWriter.Flush()
	manifest.manifestFile.Sync()

	return true
}

func GetCurrentManifest(dataDir string) (*os.File, uint64, int) {

	dirEntries, err := filepath.Glob(filepath.Join(dataDir))
	if err != nil {
		panic(err)
	}
	if len(dirEntries) == 0 {
		fileNum := uint64(1)
		manFilename := fmt.Sprintf("%016d", fileNum) + "_" + MANIFEST_FILE_SUFFIX
		manifestFile, err := os.OpenFile(filepath.Join(dataDir, manFilename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("open new manifest file err", err)
		}
		fileNumBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(fileNumBytes, fileNum)
		currfile_err := os.WriteFile(filepath.Join(dataDir, CURRENT_POINTER_FILE), fileNumBytes, 0644)
		if currfile_err != nil {
			fmt.Println("open new man curret pointer file err", currfile_err)
		}
		return manifestFile, fileNum, 0
	}
	manifestNum, err := GetCurrentManifestNumFromCurrent(dataDir)
	manFileName := fmt.Sprintf("%016d", manifestNum) + "_" + MANIFEST_FILE_SUFFIX
	manifestFile, err := os.OpenFile(manFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("open new manifest file err", err)
	}
	info, _ := manifestFile.Stat()
	return manifestFile, manifestNum, int(info.Size())

}

func GetCurrentManifestNumFromCurrent(dataDir string) (uint64, error) {
	currPtrFilename := filepath.Join(dataDir, CURRENT_POINTER_FILE)

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
