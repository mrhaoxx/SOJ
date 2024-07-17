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
					subtime := time.Now()

					id := strconv.Itoa(int(subtime.UnixNano()))
					ctx := SubmitCtx{
						ID:      id,
						Problem: pid,
						problem: &pb,
						User:    s.User(),

						SubmitTime: subtime.UnixNano(),

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

					WriteResult(uf, ctx.JudgeResult)

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
				case "status":
					if len(cmds) != 2 {
						uf.Println(aurora.Red("error:"), "invalid arguments")
						uf.Println("usage: status <submit_id>")
						return
					}

					var submit SubmitCtx
					tx := db.Where("id = ?", cmds[1]).First(&submit)
					if tx.Error != nil {
						uf.Println(aurora.Red("error:"), "submit", aurora.Yellow(strconv.Quote(cmds[1])), "not found")
						return
					}

					if submit.Status == "completed" {
						WriteResult(uf, submit.JudgeResult)
					} else {
						uf.Println("Submit", aurora.Bold(submit.ID), "is", ColorizeStatus(submit.Status))
					}

					uf.Println("\nLogs for", aurora.Bold(submit.ID))

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

func WriteResult(uf Userface, res JudgeResult) {
	if res.Success {
		uf.Println(aurora.Green("Accepted"), aurora.Bold(res.Score))
	} else {
		uf.Println(aurora.Red("Wrong Answer"), aurora.Bold(res.Score))
	}

	uf.Println("Message:\n", aurora.Cyan(res.Msg))
}
