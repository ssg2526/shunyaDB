package models

type KVInput struct {
	Op    uint8
	Key   string
	Value string
}

type KVOutput struct {
	Value string
}
