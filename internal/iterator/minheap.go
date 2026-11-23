package iterator

import (
	"github.com/ssg2526/shunya/internal/constants"
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
	h := MinHeap{
		minheap: make([]Iterator, 0, len(iteratorArray)),
	}

	for _, it := range iteratorArray {
		if it != nil && it.Valid() {
			h.minheap = append(h.minheap, it)
		}
	}
	h.Size = len(h.minheap)

	for i := h.Size/2 - 1; i >= 0; i-- {
		h.heapify(i)
	}
	return &h
}

func (itmh *MinHeap) heapify(i int) {
	if i >= itmh.Size {
		return
	}
	left := itmh.left(i)
	right := itmh.right(i)

	if left < itmh.Size {
		if constants.CompareInternalKey(itmh.minheap[left].Key(), itmh.minheap[i].Key()) < 0 {
			itmh.swap(i, left)
			itmh.heapify(left)
		}
	}
	if right < itmh.Size {
		if constants.CompareInternalKey(itmh.minheap[right].Key(), itmh.minheap[i].Key()) < 0 {
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
