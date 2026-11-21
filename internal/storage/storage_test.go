package storage

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/ssg2526/shunya/config"
	"github.com/ssg2526/shunya/internal/constants"
)

func testConfig(t *testing.T) {
	dir := t.TempDir()
	config.ShunyaConfigs = &config.ShunyaDBConfig{
		WALDir:                   filepath.Join(dir, "wal"),
		DataDir:                  filepath.Join(dir, "data"),
		ManifestDir:              filepath.Join(dir, "manifest"),
		WALShouldFsync:           false,
		WALBufSyncIntervalMillis: 1000,
		WALMaxSegmentSize:        1 << 20,
		WALWriteBufferSize:       1 << 16,
		MemTableType:             "skiplist",
		SSTWriteBufferSize:       1 << 16,
		FlushQueueSize:           10,
		IdleFlushIntervalMillis:  60_000,
		ManifestWriteBufferSize:  4096,
		MaxSSTLevel:              10,
		OpenSstLRUSize:           100,
		BlockCacheSizeByte:       8 << 20,
		SSTIndexCacheSizeByte:    1 << 20,
	}

	for _, d := range []string{config.ShunyaConfigs.WALDir, config.ShunyaConfigs.DataDir, config.ShunyaConfigs.ManifestDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

}

func put(s *Storage, k, v []byte) constants.LsnType {
	lsn := s.AppendToWal(EncodeWalCommand(constants.PutEntry, k, v))
	s.Put(k, v, lsn)
	return lsn
}

func del(s *Storage, k []byte) constants.LsnType {
	lsn := s.AppendToWal(EncodeWalCommand(constants.DelEntry, k, nil))
	s.Del(k, lsn)
	return lsn
}

func TestRandomOpsAgainstModel(t *testing.T) {
	testConfig(t)

	seed := uint64(rand.Int64())
	t.Logf("seed=%d", seed)
	r := rand.New(rand.NewPCG(seed, seed))

	s, err := InitStorage()
	if err != nil {
		t.Fatal(err)
	}

	validMap := map[string][]byte{}
	const numKeys = 2000
	const numOps = 50_000
	var lastLsn constants.LsnType
	countPassW := 0
	countPassD := 0
	countPassR := 0
	countRR := 0

	check := func(k string) {
		got := s.Get([]byte(k), lastLsn)
		want, ok := validMap[k]
		if !ok && got != nil {
			fmt.Printf("countR: %d, countW: %d, countD: %d, countRR: %d\n", countPassR, countPassW, countPassD, countRR)
			t.Fatalf("key %q: want not found, got %q (seed=%d)", k, got, seed)
		}
		if ok && !bytes.Equal(got, want) {
			fmt.Printf("countR: %d, countW: %d, countD: %d\n", countPassR, countPassW, countPassD)
			t.Fatalf("key %q: want %q, got %q (seed=%d)", k, want, got, seed)
		}
		if len(got) != 0 {
			countRR++
		}
	}

	for i := 0; i < numOps; i++ {
		k := fmt.Sprintf("key-%05d", r.IntN(numKeys))
		switch p := r.IntN(100); {
		case p < 60:
			v := fmt.Appendf(nil, "val-%d-%s", i, bytes.Repeat([]byte("x"), r.IntN(200)))
			lastLsn = put(s, []byte(k), v)
			validMap[k] = v
			countPassW++
		case p < 75:
			lastLsn = del(s, []byte(k))
			delete(validMap, k)
			countPassD++
		default:
			check(k)
			countPassR++
		}
	}
	for i := 0; i < numKeys; i++ {
		check(fmt.Sprintf("key-%05d", i))
	}
}
