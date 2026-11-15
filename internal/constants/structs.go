package constants

import (
	"bytes"
	"cmp"
)

type InternalKey struct {
	Key       []byte
	Lsn       LsnType
	EntryType EntryType
}

func CompareInternalKey(a, b InternalKey) int {
	if c := bytes.Compare(a.Key, b.Key); c != 0 {
		return c
	}
	return cmp.Compare(b.Lsn, a.Lsn)
}
