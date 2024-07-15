package main

import (
	"context"
	"log"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

var docker_cli *client.Client

func RunImage(name string, user string, hostname string, image string, workdir string, mounts []mount.Mount, mask bool, ReadonlyRootfs bool) (ok bool, id string) {

	var masked []string
	if mask {
		masked = []string{"/etc", "/sys", "/proc/tty", "/proc/sys", "/proc/sysrq-trigger", "/proc/cmdline", "/proc/config.gz", "/proc/mounts", "/proc/fs", "/proc/device-tree", "/proc/bus"}
	}

	resp, err := docker_cli.ContainerCreate(context.Background(), &container.Config{
		Image:      image,
		User:       user,
		Hostname:   hostname,
		WorkingDir: workdir,
	}, &container.HostConfig{
		MaskedPaths:    masked,
		Mounts:         mounts,
		ReadonlyRootfs: ReadonlyRootfs,
		AutoRemove:     true,
	}, nil, nil, name)

	if err != nil {
		log.Println(name, "container create error", err)
		return false, ""
	}

	id = resp.ID
	log.Println(name, "container created", id)

	err = docker_cli.ContainerStart(context.Background(), id, container.StartOptions{})

	if err != nil {
		log.Println("container start error", err)
		return false, ""
	}

	log.Println(name, "container started", id)

waiting:
	info, err := docker_cli.ContainerInspect(context.Background(), id)
	if err != nil {
		panic(err)
	}

	if !info.State.Running {
		time.Sleep(50 * time.Millisecond)
		log.Println(name, "container is not running,waiting", info.State.Status)
		goto waiting
	}

	log.Println(name, "container is running", id)

	return true, id
}

func CleanContainer(id string) {
	err := docker_cli.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	if err != nil {
		log.Println("container remove error", err)
	}
	log.Println("container removed", id)
}

func GetContainerIP(id string) string {
	info, err := docker_cli.ContainerInspect(context.Background(), id)
	if err != nil {
		panic(err)
	}

	return info.NetworkSettings.IPAddress
}

func ExecContainer(id string, cmd string, timeout int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	resp, err := docker_cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd: []string{"sh", "-c", cmd},
	})

	if err != nil {
		log.Println("container exec create error", err)
		return err
	}

	log.Println("container exec created", resp.ID)

	err = docker_cli.ContainerExecStart(ctx, resp.ID, container.ExecStartOptions{
		Detach: false,
	})
	if err != nil {
		log.Println("container exec start error", err)
		return err
	}

	log.Println("container exec started", resp.ID)

	return nil
}
