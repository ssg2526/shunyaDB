package skiplist

import (
	"math/rand"

	constants "github.com/ssg2526/shunya/internal/constants"
)

type Node struct {
	key       string
	value     string
	entryType constants.EntryType
	lsn       constants.LsnType
	lvlPtrs   []*Node
}

type Skiplist struct {
	threshold float64
	maxLevel  int
	head      *Node
	size      int
}

//TODO: Need to do handle concurrent cases for correctness

func NewNode(key string, value string, lsn constants.LsnType, entryType constants.EntryType, lvl int) *Node {
	return &Node{key: key, value: value, lsn: lsn, lvlPtrs: make([]*Node, lvl+1)}
}

func NewSkiplist(threshold float64, maxLevel int) *Skiplist {
	return &Skiplist{
		head: &Node{
			key:     "",
			value:   "",
			lvlPtrs: make([]*Node, maxLevel),
		},
		threshold: threshold,
		maxLevel:  maxLevel}
}

func compareString(s1 string, s2 string) int {
	// TODO: this needs optimizations
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}

func shouldMoveToNextLvl(threshold float64) bool {
	rand_toss := rand.Float64()
	return rand_toss > threshold
}

// func (skipList *Skiplist) findNearestNodeToKey(key string) ([]*Node, bool) {

// 	return nodeList, false
// }

func (skipList *Skiplist) Get(key string) (value string) {
	curr := skipList.head

	for i := skipList.maxLevel - 1; i >= 0; i-- {

		for curr.lvlPtrs[i] != nil && compareString(curr.lvlPtrs[i].key, key) < 0 {
			curr = curr.lvlPtrs[i]
		}

		if curr.lvlPtrs[i] != nil && compareString(curr.lvlPtrs[i].key, key) == 0 {
			return curr.lvlPtrs[i].value
		}
	}
	return "nil"
}

func (skipList *Skiplist) Put(key string, value string, lsn constants.LsnType, entryType constants.EntryType) {
	nodeList := make([]*Node, skipList.maxLevel)

	curr := skipList.head

	for i := skipList.maxLevel - 1; i >= 0; i-- {
		for curr.lvlPtrs[i] != nil && compareString(curr.lvlPtrs[i].key, key) < 0 {
			curr = curr.lvlPtrs[i]
		}
		// NOTE: Early return on key found at higher levels.
		// This assumes skiplist invariant holds strictly.
		// Revisit when adding concurrency or lock-free traversal and check if we need to go to level 0 and then update
		if curr.lvlPtrs[i] != nil && compareString(curr.lvlPtrs[i].key, key) == 0 {
			curr.lvlPtrs[i].lsn = lsn
			curr.lvlPtrs[i].value = value
			curr.lvlPtrs[i].entryType = entryType
			return
		}
		nodeList[i] = curr
	}

	insertLvl := 0

	for shouldMoveToNextLvl(skipList.threshold) {
		insertLvl++
		if insertLvl >= skipList.maxLevel {
			insertLvl = skipList.maxLevel - 1
			break
		}
	}

	node := NewNode(key, value, lsn, entryType, insertLvl)

	for i := len(nodeList) - 1; i >= 0; i-- {
		if i <= insertLvl {
			node.lvlPtrs[i] = nodeList[i].lvlPtrs[i]
			nodeList[i].lvlPtrs[i] = node
		}
	}
	skipList.size += len(key) + len(value) + 8
}

func (skiplist *Skiplist) Size() int {
	return skiplist.size
}
