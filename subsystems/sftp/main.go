package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/mrhaoxx/sftp"
)

func main() {

	lc, err := net.Listen("tcp", "0.0.0.0:2207")
	if err != nil {
		log.Fatalf("failed to listen on 0.0.0.0:2207")
	}

	log.Println("sftp server listening")

	conn, err := lc.Accept()
	if err != nil {
		log.Fatalf("failed to accept connection")
	}

	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
		sftp.WithServerWorkingDirectory("/work"),
	}
	server, err := sftp.NewServer(
		conn,
		serverOptions...,
	)
	if err != nil {
		log.Printf("sftp server init error: %s\n", err)
		return
	}
	log.Println("sftp server working")

	if err := server.Serve(); err == io.EOF {
		server.Close()
		fmt.Println("sftp client exited session.")
	} else if err != nil {
		fmt.Println("sftp server completed with error:", err)
	}
}
