package storage

import (
	"encoding/binary"

	constants "github.com/ssg2526/shunya/internal/constants"
)

const (
	walCmdOpSize     = 1
	walCmdKeyLenSize = 4
	walCmdValLenSize = 4
)

func EncodeWalCommand(op constants.EntryType, key []byte, value []byte) []byte {
	totalSize := walCmdOpSize + walCmdKeyLenSize + len(key) + walCmdValLenSize + len(value)
	encoded := make([]byte, totalSize)

	offset := 0
	encoded[offset] = byte(op)
	offset += walCmdOpSize

	binary.LittleEndian.PutUint32(encoded[offset:], uint32(len(key)))
	offset += walCmdKeyLenSize
	copy(encoded[offset:], key)
	offset += len(key)

	binary.LittleEndian.PutUint32(encoded[offset:], uint32(len(value)))
	offset += walCmdValLenSize
	copy(encoded[offset:], value)

	return encoded
}

func DecodeWalCommand(data []byte) (op constants.EntryType, key []byte, value []byte) {
	offset := 0
	op = constants.EntryType(data[offset])
	offset += walCmdOpSize

	keyLen := binary.LittleEndian.Uint32(data[offset:])
	offset += walCmdKeyLenSize
	key = data[offset : offset+int(keyLen)]
	offset += int(keyLen)

	valLen := binary.LittleEndian.Uint32(data[offset:])
	offset += walCmdValLenSize
	value = data[offset : offset+int(valLen)]

	return op, key, value
}
