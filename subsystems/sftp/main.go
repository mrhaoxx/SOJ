package main

import (
	"io"
	"log"
	"net"
	"net/netip"
	"os"

	"github.com/mrhaoxx/sftp"
)

func main() {

	defer func() {
		if r := recover(); r != nil {
			log.Println("recovered from panic", r)
		}
	}()

	lc, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.MustParseAddrPort("0.0.0.0:2207")))
	if err != nil {
		log.Fatalf("failed to listen on 0.0.0.0:2207")
	}

	log.Println("sftp server listening")

	conn, err := lc.AcceptTCP()
	if err != nil {
		log.Fatalf("failed to accept connection")
	}

	// debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(os.Stdout),
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
		log.Println("sftp client exited session.")
	} else if err != nil {
		log.Println("sftp server completed with error:", err)
	}

	log.Println("sftp server exited")
}
