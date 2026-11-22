package storage

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/ssg2526/shunya/internal/constants"
)

// expectedRange returns model values for keys in [start, end), in key order.
func expectedRange(model map[string][]byte, start, end []byte) [][]byte {
	keys := make([]string, 0, len(model))
	for k := range model {
		if k >= string(start) && (end == nil || k < string(end)) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	out := make([][]byte, len(keys))
	for i, k := range keys {
		out[i] = model[k]
	}
	return out
}

// scanAll follows continuation tokens until the range is exhausted.
func scanAll(t *testing.T, s *Storage, start, end []byte, limit int, lsn constants.LsnType) []constants.RangeData {
	var all []constants.RangeData
	for pages := 0; ; pages++ {
		resp := s.Range(start, end, limit, lsn)
		if len(resp.Data) > limit {
			t.Fatalf("page has %d items, limit %d", len(resp.Data), limit)
		}
		if len(resp.Data) < 2 {
			fmt.Printf("data smaller than expected: %d, %d\n", len(resp.Data), limit)
		}
		if pages > 10_000 {
			fmt.Printf("len, key, val: %d, %s, %s, %s, %s\n", len(resp.Data), resp.Data[0].Key, resp.Data[0].Value, resp.Data[len(resp.Data)-1].Key, resp.Data[len(resp.Data)-1].Value)
			t.Fatalf("pagination did not terminate")
		}
		all = append(all, resp.Data...)
		if resp.ContinuationToken == nil {
			return all
		}
		start = resp.ContinuationToken.Key
	}
}

func TestRangeAgainstModel(t *testing.T) {
	testConfig(t)
	seed := uint64(rand.Int64())
	t.Logf("seed=%d", seed)
	r := rand.New(rand.NewPCG(seed, seed))

	s, err := InitStorage()
	if err != nil {
		t.Fatal(err)
	}

	model := map[string][]byte{}
	const numKeys = 2000
	var lastLsn constants.LsnType
	key := func(i int) []byte { return fmt.Appendf(nil, "key-%05d", i) }

	checkRange := func(op int, start, end []byte, limit int) {
		want := expectedRange(model, start, end)
		got := scanAll(t, s, start, end, limit, lastLsn)
		if len(got) != len(want) {
			t.Fatalf("op %d range [%s,%s) limit %d: got %d items, want %d (seed=%d)", op, start, end, limit, len(got), len(want), seed)
		}
		for j := range want {
			if !bytes.Equal(got[j].Value, want[j]) {
				t.Fatalf("op %d range [%s,%s): item %d got %.20q want %.20q (seed=%d)", op, start, end, j, got[j], want[j], seed)
			}
		}
	}

	// op mix in percent; whatever is left after put + del is range
	const numOps = 30_000
	const putPct, delPct = 55, 15 // → 30% range calls

	for i := 0; i < numOps; i++ {
		k := key(r.IntN(numKeys))
		switch p := r.IntN(100); {
		case p < putPct:
			v := fmt.Appendf(nil, "val-%d-%s", i, bytes.Repeat([]byte("x"), r.IntN(200)))
			lastLsn = put(s, k, v)
			model[string(k)] = v
		case p < putPct+delPct:
			lastLsn = del(s, k)
			delete(model, string(k))
		default:
			a := r.IntN(numKeys)
			width := 1 + r.IntN(100)
			if r.IntN(20) == 0 {
				width = numKeys
			}
			checkRange(i, key(a), key(min(a+width, numKeys)), 1+r.IntN(50))
		}
	}

	// final sweep: whole key space with a few page sizes, open-ended end key
	for _, limit := range []int{1, 7, 100} {
		checkRange(numOps, key(0), nil, limit)
	}
}
