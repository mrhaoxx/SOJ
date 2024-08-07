package main

import (
	"bytes"
	"context"
	"io"

	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/rs/zerolog/log"
)

var docker_cli *client.Client

func RunImage(name string, user string, hostname string, image string, workdir string, mounts []mount.Mount, mask bool, ReadonlyRootfs bool, networkdisabled bool, timeout int, networkhosted bool, env []string) (ok bool, id string) {

	var masked []string
	if mask {
		masked = []string{"/etc", "/sys", "/proc/tty", "/proc/sys", "/proc/sysrq-trigger", "/proc/cmdline", "/proc/config.gz", "/proc/mounts", "/proc/fs", "/proc/device-tree", "/proc/bus"}
	}

	network := ""
	if networkhosted {
		network = "host"
	}

	resp, err := docker_cli.ContainerCreate(context.Background(), &container.Config{
		Image:           image,
		User:            user,
		Hostname:        hostname,
		WorkingDir:      workdir,
		NetworkDisabled: networkdisabled,
		Env:             env,
		StopTimeout:     &timeout,
	}, &container.HostConfig{
		MaskedPaths:    masked,
		Mounts:         mounts,
		ReadonlyRootfs: ReadonlyRootfs,
		AutoRemove:     true,
		NetworkMode:    container.NetworkMode(network),

		Resources: container.Resources{Ulimits: []*container.Ulimit{
			{Name: "memlock", Soft: -1, Hard: -1},
		}},
	}, nil, nil, name)

	if err != nil {
		// log.Println(name, "container create error", err)
		log.Err(err).Str("name", name).Str("image", image).Msg("container create error")
		return false, ""
	}

	id = resp.ID

	log.Debug().Str("name", name).Str("image", image).Str("id", id).Msg("container created")

	err = docker_cli.ContainerStart(context.Background(), id, container.StartOptions{})

	if err != nil {
		log.Err(err).Str("name", name).Str("image", image).Str("id", id).Msg("container start error")
		return false, ""
	}

	log.Debug().Str("name", name).Str("image", image).Str("id", id).Msg("container started")

	return true, id
}

func CleanContainer(id string) {
	var timeout = 1
	err := docker_cli.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &timeout})
	// err := docker_cli.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil {
		log.Err(err).Str("id", id).Msg("container remove error")
		return
	}
	log.Debug().Str("id", id).Msg("container removed")
}

func GetContainerIP(id string) string {
	info, err := docker_cli.ContainerInspect(context.Background(), id)
	if err != nil {
		log.Err(err).Str("id", id).Msg("failed to get ip: container inspect error")
		return ""
	}

	return info.NetworkSettings.IPAddress
}

func ExecContainer(id string, cmd string, timeout int, stdout, stderr io.Writer, env []string, privileged bool) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	resp, err := docker_cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", cmd},
		Env:          env,
		Privileged:   privileged,
	})

	if err != nil {
		log.Err(err).Str("id", id).Msg("container exec create error")
		return -1, "", err
	}

	log.Debug().Str("id", id).Str("exec_id", resp.ID).Msg("container exec created")

	outresp, err := docker_cli.ContainerExecAttach(ctx, resp.ID, container.ExecStartOptions{})
	if err != nil {
		log.Err(err).Str("id", id).Str("exec_id", resp.ID).Msg("container exec attach error")
		return -1, "", err
	}
	defer outresp.Close()

	log.Debug().Str("id", id).Str("exec_id", resp.ID).Msg("container exec started")

	buf := bytes.NewBuffer(nil)
	if stdout != nil && stderr != nil {
		_, err := stdcopy.StdCopy(stdout, stderr, io.TeeReader(outresp.Reader, buf))
		if err != nil {
			log.Err(err).Str("id", id).Str("exec_id", resp.ID).Msg("container exec copy error")
		}
	} else {
		_, err = io.Copy(buf, outresp.Reader)
		if err != nil {
			log.Err(err).Str("id", id).Str("exec_id", resp.ID).Msg("container exec copy error")
		}
	}

	inspectResp, err := docker_cli.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		log.Err(err).Str("id", id).Str("exec_id", resp.ID).Msg("container exec inspect error")
		return -1, "", err
	}

	return inspectResp.ExitCode, buf.String(), err
}

func GetContainerLogs(id string) (string, error) {
	resp, err := docker_cli.ContainerLogs(context.Background(), id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		log.Err(err).Str("id", id).Msg("container logs error")
		return "", err
	}

	res, err := io.ReadAll(resp)

	if err != nil {
		log.Err(err).Str("id", id).Msg("container logs read error")
		return "", err
	}

	return string(res), nil
}
