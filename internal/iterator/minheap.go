package iterator

import (
	"bytes"
)

type MinHeap struct {
	Size    int
	minheap []Iterator
}

func (itmh *MinHeap) Peek() Iterator {
	if itmh.Size <= 0 {
		return nil
	}
	return itmh.minheap[0]
}

func (itmh *MinHeap) GetMin() Iterator {
	if itmh.Size <= 0 {
		return nil
	}
	minIterator := itmh.minheap[0]
	if itmh.minheap[0].Valid() {
		itmh.minheap[0].Next()
	}

	if !itmh.minheap[0].Valid() {
		itmh.minheap[0] = itmh.minheap[itmh.Size-1]
		itmh.Size--
	}
	itmh.heapify(0)
	return minIterator
}

func NewMinHeapFromArray(iteratorArray []Iterator) *MinHeap {
	itmh := MinHeap{
		Size:    len(iteratorArray),
		minheap: iteratorArray,
	}

	i := 0
	j := itmh.Size - 1

	for i < j {
		for i <= j && (itmh.minheap[j] == nil || !itmh.minheap[j].Valid()) {
			j--
		}
		if itmh.minheap[i] == nil || !itmh.minheap[i].Valid() {
			itmh.swap(i, j)
		}
		i++
	}

	itmh.Size = j + 1

	for i := itmh.Size - 2; i >= 0; i-- {
		itmh.heapify(i)
	}
	return &itmh
}

func (itmh *MinHeap) heapify(i int) {
	if i >= itmh.Size {
		return
	}
	left := itmh.left(i)
	right := itmh.right(i)

	if left < itmh.Size {
		comp := bytes.Compare(itmh.minheap[left].Key().Key, itmh.minheap[i].Key().Key)
		if comp < 1 || (comp == 0 && itmh.minheap[left].Key().Lsn > itmh.minheap[i].Key().Lsn) {
			itmh.swap(i, left)
			itmh.heapify(left)
		}
	}
	if right < itmh.Size {
		comp := bytes.Compare(itmh.minheap[right].Key().Key, itmh.minheap[i].Key().Key)
		if comp < 1 || (comp == 0 && itmh.minheap[right].Key().Lsn > itmh.minheap[i].Key().Lsn) {
			itmh.swap(i, right)
			itmh.heapify(right)
		}
	}
}

func (itmh *MinHeap) left(i int) int {
	return 2*i + 1
}

func (itmh *MinHeap) right(i int) int {
	return 2*i + 2
}

func (itmh *MinHeap) swap(i, j int) {
	tmp := itmh.minheap[i]
	itmh.minheap[i] = itmh.minheap[j]
	itmh.minheap[j] = tmp
}
