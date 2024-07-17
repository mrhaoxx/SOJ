package main

import (
	"bytes"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/logrusorgru/aurora/v4"
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

	db.AutoMigrate(&SubmitCtx{})

	problems := LoadProblemDir(cfg.ProblemsDir)

	s := &ssh.Server{
		Addr: cfg.ListenAddr,
		Handler: func(s ssh.Session) {
			uf := Userface{
				Buffer: bytes.NewBuffer(nil),
				Writer: s,
			}

			cmds := s.Command()
			log.Info().Str("user", s.User()).Strs("cmds", cmds).Msg("new session")

			uf.Println("Welcome to", aurora.Bold("SOJ"), aurora.Gray(aurora.GrayIndex(10), "Secure Online Judge"))
			uf.Println(aurora.Yellow(time.Now().Format(time.DateTime + " MST")))

			if len(cmds) == 0 {

				uf.Println("Type 'submit <problem_id>' to submit a problem")

			} else {
				switch cmds[0] {
				case "submit":
					if len(cmds) != 2 {
						uf.Println(aurora.Red("error:"), "invalid arguments")
						uf.Println("usage: submit <problem_id>")
						return
					}

					pid := cmds[1]

					pb, ok := problems[pid]
					if !ok {
						uf.Println(aurora.Red("error:"), "problem", aurora.Yellow(strconv.Quote(pid)), "not found")
						return
					}

					uf.Println(aurora.Green("Submitting"), aurora.Bold(pid))

					id := time.Now().Format("20060102150405") + strconv.Itoa(os.Getpid())
					ctx := SubmitCtx{
						ID:      id,
						Problem: pid,
						problem: &pb,
						User:    s.User(),

						SubmitTime: time.Now().UnixNano(),

						Status: "init",

						SubmitDir: path.Join(cfg.SubmitsDir, s.User(), pid),
						Workdir:   path.Join(cfg.SubmitWorkDir, id),

						Userface: Userface{
							Buffer: bytes.NewBuffer(nil),
							Writer: uf,
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

				case "history":
					if len(cmds) != 1 {
						uf.Println(aurora.Red("error:"), "invalid arguments")
						uf.Println("usage: history")
						return
					}

					var submits []SubmitCtx
					db.Where("user = ?", s.User()).Find(&submits)

					uf.Println("Your submissions:")
					for _, submit := range submits {
						uf.Println(submit.ID, submit.Problem, submit.Status, submit.Msg, submit.JudgeResult.Score)
					}
				case "logs":
					if len(cmds) != 2 {
						uf.Println(aurora.Red("error:"), "invalid arguments")
						uf.Println("usage: logs <submit_id>")
						return
					}

					var submit SubmitCtx
					tx := db.Where("id = ?", cmds[1]).First(&submit)
					if tx.Error != nil {
						uf.Println(aurora.Red("error:"), "submit", aurora.Yellow(strconv.Quote(cmds[1])), "not found")
						return
					}

					uf.Println("Logs for", aurora.Bold(submit.ID))

					s.Write(submit.Userface.Buffer.Bytes())

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
