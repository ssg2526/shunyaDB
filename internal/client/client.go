package client

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
)

func Start() {

	raddr, _ := net.ResolveTCPAddr("tcp", "localhost:4242")
	conn, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		fmt.Println("error connecting to server", err)
	}
	handleConnection(conn)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buff := make([]byte, 1024)
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n> ")
		inputBytes, errRead := reader.ReadBytes('\n')
		if errRead != nil {
			fmt.Print("error reading data", inputBytes)
			continue
		}
		inputBytes = bytes.TrimSpace(inputBytes)

		_, errWrite := conn.Write(inputBytes)
		if errWrite != nil {
			fmt.Print("error sending command", errWrite)
			continue
		}
		bytesRed, errRes := conn.Read(buff)
		if errRes != nil {
			fmt.Print("error reading command result", errRes)
			continue
		}
		fmt.Printf("%s", buff[:bytesRed])
	}
}
