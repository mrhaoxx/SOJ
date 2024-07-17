package main

import (
	"context"
	"io"
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

func ExecContainer(id string, cmd string, timeout int) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	resp, err := docker_cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", cmd},
	})

	if err != nil {
		log.Println("container exec create error", err)
		return -1, "", err
	}

	log.Println("container exec created", resp.ID)

	outresp, err := docker_cli.ContainerExecAttach(ctx, resp.ID, container.ExecStartOptions{})
	if err != nil {
		log.Println("container exec attach error", err)
		return -1, "", err
	}
	defer outresp.Close()

	log.Println("container exec started", resp.ID)

	outs, err := io.ReadAll(outresp.Reader)

	inspectResp, err := docker_cli.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		log.Println("container exec inspect error", err)
		return -1, "", err
	}

	return inspectResp.ExitCode, string(outs), err
}

func GetContainerLogs(id string) (string, error) {
	resp, err := docker_cli.ContainerLogs(context.Background(), id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		log.Println("container logs error", err)
		return "", err
	}

	res, err := io.ReadAll(resp)

	if err != nil {
		log.Println("container logs read error", err)
		return "", err
	}

	return string(res), nil
}
