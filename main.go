package main

import (
	"log"
	"os"

	"github.com/docker/docker/client"
	ssh "github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

type Config struct {
	HostKey    string `yaml:"HostKey"`
	ListenAddr string `yaml:"ListenAddr"`

	SubmitsDir    string `yaml:"SubmitsDir"`
	SubmitWorkDir string `yaml:"SubmitWorkDir"`
	ProblemsDir   string `yaml:"ProblemsDir"`

	DockerCli string `yaml:"DockerCli"`
}

var cfg = Config{}

func main() {
	_cfg, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal("failed to read config file", err)
	}

	err = yaml.Unmarshal(_cfg, &cfg)
	if err != nil {
		log.Fatal("failed to parse config file", err)
	}

	docker_cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatal("failed to create docker client", err)
	}

	pk, err := gossh.ParsePrivateKey([]byte(cfg.HostKey))
	if err != nil {
		log.Fatal(err)
	}

	s := &ssh.Server{
		Addr: cfg.ListenAddr,
		Handler: func(s ssh.Session) {
			log.Println("new session", s.User())
			s.Write([]byte("Welcome to Secure Online Judge\n"))

			res := SubmitJudge(s.User(), Problem{
				Submits: []Submit{
					{
						Path:  "hello.txt",
						IsDir: false,
					}},
				Workflow: []Workflow{
					{
						Image: "docker.io/library/soj-subsystem-shell",
						Steps: []string{
							"ls / > /work/world.txt",
							"echo -n {\\\"Success\\\":true,\\\"Msg\\\":\\\" Your submit context is >> /work/result.json",
							"cat /submit/hello.txt >> /work/hello.txt",
							"echo \\\"} >> /work/result.json",
						},
						Timeout: 1800,
					},
				},
			})

			if res.Success {
				s.Write([]byte("judge success\n"))
				s.Write([]byte(res.Msg))
			} else {
				s.Write([]byte("judge failed\n"))
			}

			log.Println(res)
			log.Println("session done", s.User())

		},
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": SftpHandler,
		},
	}
	s.AddHostKey(pk)

	log.Println("listening on", cfg.ListenAddr)
	log.Fatal(s.ListenAndServe())
}
