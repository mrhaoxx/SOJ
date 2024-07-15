package main

import (
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/docker/docker/api/types/mount"
	ssh "github.com/gliderlabs/ssh"
)

// SftpHandler handler for SFTP subsystem
func SftpHandler(sess ssh.Session) {
	name := "soj-subsystem-sftp-" + sess.User() + "-" + time.Now().Format("20060102150405")
	path := cfg.SubmitsDir + "/" + sess.User()
	log.Println("new sftp session", sess.User(), name, path)

	if err := os.MkdirAll(path, 0700); err != nil {
		log.Println(name, "failed to create working dir", path, err)
		return
	}

	success, id := RunImage(name, "999", "soj-sftpd", "docker.io/mrhaoxx/soj-subsystem-sftp", "/", []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: path,
			Target: "/work",
		},
	}, true, true)

	if !success {
		log.Println(name, "failed to run sftp container")
		return
	}
	defer CleanContainer(id)

	time.Sleep(200 * time.Millisecond)

	ip := GetContainerIP(id)

	conn, err := net.Dial("tcp", ip+":2207")
	if err != nil {
		log.Println(name, "failed to connect to container", id, err)
		return
	}

	log.Println(name, "connected to container", id)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		io.Copy(conn, sess)
		conn.Close()
		wg.Done()
	}()
	go func() {
		io.Copy(sess, conn)
		sess.CloseWrite()
		wg.Done()
	}()

	wg.Wait()

	log.Println(name, "session closed", id)
}
