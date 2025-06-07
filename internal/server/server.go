package server

import (
	"fmt"
	"net"
)

func Start() {
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
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buff := make([]byte, 1024)
	for {
		bytesRed, err := conn.Read(buff)
		if err != nil {
			fmt.Println("read error:", err)
			break
		}
		fmt.Printf("recvd: %s", buff[:bytesRed])

		_, errWrite := conn.Write([]byte("OK"))
		if errWrite != nil {
			fmt.Print("error while sending ack")
		}
	}
}
