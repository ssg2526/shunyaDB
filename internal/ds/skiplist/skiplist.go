package skiplist

import (
	"math/rand"
)

type Node struct {
	key     string
	value   string
	lvlPtrs []*Node
}

type Skiplist struct {
	threshold float64
	maxLevel  int
	head      *Node
}

func NewNode(key string, value string, lvl int) *Node {
	return &Node{key: key, value: value, lvlPtrs: make([]*Node, lvl)}
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

func (skipList *Skiplist) Get(key string) (value string) {
	curr := skipList.head

	for i := skipList.maxLevel - 1; i >= 0; i-- {

		strCompare := compareString(curr.lvlPtrs[i].key, key)

		for curr.lvlPtrs[i] != nil && strCompare < 0 {
			curr = curr.lvlPtrs[i]
		}

		if curr.lvlPtrs[i] != nil && strCompare == 0 {
			return curr.lvlPtrs[i].value
		}
	}
	return ""
}

func (skipList *Skiplist) Insert(key string, value string) {
	insertLvl := 0

	for shouldMoveToNextLvl(skipList.threshold) {
		insertLvl++
		if insertLvl >= skipList.maxLevel {
			insertLvl = skipList.maxLevel - 1
			break
		}
	}
	node := NewNode(key, value, insertLvl)

	curr := skipList.head

	for i := skipList.maxLevel - 1; i >= 0; i-- {

		strCompare := compareString(curr.lvlPtrs[i].key, key)

		for curr.lvlPtrs[i] != nil && strCompare < 0 {
			curr = curr.lvlPtrs[i]
		}
		if curr.lvlPtrs[i] == nil {
			if i <= insertLvl {
				curr.lvlPtrs[i] = node
			}
		} else if strCompare > 0 && i <= insertLvl {
			node.lvlPtrs[i] = curr.lvlPtrs[i]
			curr.lvlPtrs[i] = node
		}
	}
}

func (skipList *Skiplist) Delete(key string) bool {
	found := false
	curr := skipList.head

	for i := skipList.maxLevel - 1; i >= 0; i-- {

		strCompare := compareString(curr.lvlPtrs[i].key, key)

		for curr.lvlPtrs[i] != nil && strCompare < 0 {
			curr = curr.lvlPtrs[i]
		}

		if curr.lvlPtrs[i] != nil && strCompare == 0 {
			tmp := curr.lvlPtrs[i]
			curr.lvlPtrs[i] = tmp.lvlPtrs[i]
			tmp = nil
			found = true
		}
	}
	return found
}
