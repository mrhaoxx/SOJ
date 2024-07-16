package main

import (
	"bytes"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/docker/docker/client"
	ssh "github.com/gliderlabs/ssh"
	"github.com/rs/zerolog"
	gossh "golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	HostKey    string `yaml:"HostKey"`
	ListenAddr string `yaml:"ListenAddr"`

	SubmitsDir    string `yaml:"SubmitsDir"`
	SubmitWorkDir string `yaml:"SubmitWorkDir"`
	ProblemsDir   string `yaml:"ProblemsDir"`

	SqlitePath string `yaml:"SqlitePath"`

	DockerCli string `yaml:"DockerCli"`
}

var cfg = Config{}

func main() {

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	_cfg, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read config file")
	}

	err = yaml.Unmarshal(_cfg, &cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse config file")
	}

	docker_cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create docker client")
	}

	pk, err := gossh.ParsePrivateKey([]byte(cfg.HostKey))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse host key")
	}

	db, err = gorm.Open(sqlite.Open(cfg.SqlitePath), &gorm.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open sqlite")
	}

	db.AutoMigrate(&SubmitRecord{})

	problems := LoadProblemDir(cfg.ProblemsDir)

	s := &ssh.Server{
		Addr: cfg.ListenAddr,
		Handler: func(s ssh.Session) {
			log.Info().Str("user", s.User()).Msg("new session")
			cmds := s.Command()
			log.Info().Str("user", s.User()).Strs("cmds", cmds).Msg("command")

			if len(cmds) == 0 {
				s.Write([]byte(s.User() + ", Welcome to Secure Online Judge\n"))
			} else {
				switch cmds[0] {
				case "submit":
					if len(cmds) != 2 {
						s.Write([]byte("usage: submit <problem_id>\n"))
						return
					}
					pid := cmds[1]

					pb, ok := problems[pid]
					if !ok {
						s.Write([]byte("problem " + strconv.Quote(pid) + " not found\n"))
						return
					}

					s.Write([]byte("submitting " + strconv.Quote(pid) + "\n"))

					id := time.Now().Format("20060102150405") + strconv.Itoa(os.Getpid())
					ctx := SubmitCtx{
						ID:      id,
						Problem: pid,
						problem: &pb,
						User:    s.User(),

						SubmitTime: time.Now().UnixNano(),

						Status: "init",

						SubmitsHashes: make(map[string]string),

						SubmitDir: path.Join(cfg.SubmitsDir, s.User(), pid),
						Workdir:   path.Join(cfg.SubmitWorkDir, id),

						userface: userface{
							Buffer: bytes.NewBuffer(nil),
							Writer: s,
						},
						running: make(chan struct{}),
					}

					go RunJudge(&ctx)

					<-ctx.running

					s.Write([]byte("submission:\n"))
					s.Write([]byte(ctx.Status + "\n"))
					s.Write([]byte(ctx.Msg + "\n"))

					s.Write([]byte("results:\n"))
					s.Write([]byte("success: " + strconv.FormatBool(ctx.JudgeResult.Success) + "\n"))
					s.Write([]byte("score: " + strconv.Itoa(ctx.JudgeResult.Score) + "\n"))
					s.Write([]byte("msg: " + ctx.JudgeResult.Msg + "\n"))
				default:
					s.Write([]byte("unknown command " + strconv.Quote(s.RawCommand()) + "\n"))
				}
			}

		},
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": SftpHandler,
		},
	}
	s.AddHostKey(pk)

	log.Info().Str("addr", cfg.ListenAddr).Msg("listening")
	err = s.ListenAndServe()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}
}
