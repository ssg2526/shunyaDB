package server

import (
	"bytes"
	"errors"
	"fmt"
	"net"

	"github.com/ssg2526/shunya/config"
	constants "github.com/ssg2526/shunya/internal/constants"
	"github.com/ssg2526/shunya/internal/storage"
)

type Command int

const (
	GET Command = iota
	SET
	DEL
)

type CommandData struct {
	op     uint16
	key    []byte
	value  []byte
	params []byte
}

func Start() {
	config.InitConfig()
	storageEngine := storage.InitStorage()
	ln, err := net.Listen("tcp", ":4242")
	if err != nil {
		fmt.Println("error while starting the Shunya server")
		return
	}
	fmt.Println("Shunya server started at 4242")
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("error while accepting connection")
			continue
		}
		go handleConnection(conn, storageEngine)
	}
}

func handleConnection(conn net.Conn, storage *storage.Storage) {
	defer conn.Close()

	buff := make([]byte, 1024)
	for {
		bytesRed, err := conn.Read(buff)
		if err != nil {
			fmt.Println("read error:", err)
			break
		}
		fmt.Printf("recvd: %s\n", buff[:bytesRed])
		commandData, err := parseAndValidateCommand(buff[:bytesRed])
		if err != nil {
			conn.Write([]byte("command failed"))
			continue
		}
		var lsn constants.LsnType
		if commandData.op != uint16(GET) {
			lsn = storage.AppendToWal(buff[:bytesRed])
		}

		returnVal, _ := executeCommand(commandData, lsn, storage)
		_, errWrite := conn.Write([]byte(returnVal))
		if errWrite != nil {
			fmt.Print("error while sending ack")
		}
	}
}

func parseAndValidateCommand(inputBytes []byte) (*CommandData, error) {
	byteSplitParts := bytes.Fields(inputBytes)
	if len(byteSplitParts) == 0 {
		return nil, errors.New("empty command")
	}

	command := string(bytes.ToLower(byteSplitParts[0]))
	switch command {
	case "get":
		if len(byteSplitParts) != 2 {
			return nil, errors.New("invalid get command")
		}

		return &CommandData{
			op:     uint16(GET),
			key:    byteSplitParts[1],
			value:  nil,
			params: nil,
		}, nil

	case "set":
		if len(byteSplitParts) != 3 {
			return nil, errors.New("invalid set command")
		}

		return &CommandData{
			op:     uint16(SET),
			key:    byteSplitParts[1],
			value:  byteSplitParts[2],
			params: nil,
		}, nil

	case "del":
		if len(byteSplitParts) != 2 {
			return nil, errors.New("invalid del command")
		}

		return &CommandData{
			op:     uint16(DEL),
			key:    byteSplitParts[1],
			value:  nil,
			params: nil,
		}, nil

	default:
		return nil, errors.New("unknown command")
	}
}

func executeCommand(commandData *CommandData, lsn constants.LsnType, storage *storage.Storage) (string, error) {
	if commandData.op == uint16(GET) {
		return storage.Get(commandData.key, lsn), nil
	} else if commandData.op == uint16(SET) {
		storage.Put(commandData.key, commandData.value, lsn)
		return "OK", nil
	} else if commandData.op == uint16(DEL) {
		if storage.Del(commandData.key, lsn) == "OK" {
			return "OK", nil
		}
		return "Failed", nil
	}
	return "Failed", nil
}
