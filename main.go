package main

import (
	"bytes"
	"fmt"
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

	db.Model(&SubmitCtx{}).Where("status != ? AND status != ? AND status != ?", "completed", "dead", "failed").Update("status", "dead")

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

			if len(cmds) == 0 {
				uf.Println("Welcome to", aurora.Bold("SOJ"), aurora.Gray(aurora.GrayIndex(10), "Secure Online Judge"))
				uf.Println(aurora.Yellow(time.Now().Format(time.DateTime + " MST")))
				uf.Println("Type 'submit <problem_id>' to submit a problem")
				uf.Println("Type 'history [page]' to list your submissions")
				uf.Println("Type 'status <submit_id>' to show a submission")
				uf.Println()

			} else {
				uf.Println(aurora.Yellow(time.Now().Format(time.DateTime + " MST")))

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
						// JudgeResult: JudgeResult{Score: -1},
						running: make(chan struct{}),
					}

					go RunJudge(&ctx)

					<-ctx.running

					uf.Println("Submit", "is", ColorizeStatus(ctx.Status))
					uf.Println("Message:\n	", aurora.Blue(ctx.Msg))

					WriteResult(uf, ctx)

				case "history":
					if len(cmds) > 2 {
						uf.Println(aurora.Red("error:"), "invalid arguments")
						uf.Println("usage: history [page]")
						return
					}

					uf.Println(aurora.Green("Listing"), aurora.Bold("submissions"))

					page := 1

					if len(cmds) == 2 {
						var err error
						page, err = strconv.Atoi(cmds[1])
						if err != nil {
							uf.Println(aurora.Red("error:"), "invalid page number")
							return
						}
					}

					var submits []SubmitCtx
					//paging
					// reverse order

					// db.Where("user = ?", s.User()).Offset((page - 1) * 10).Limit(10).Find(&submits)
					db.Where("user = ?", s.User()).Order("submit_time desc").Offset((page - 1) * 10).Limit(10).Find(&submits)

					var total int64
					db.Model(&SubmitCtx{}).Where("user = ?", s.User()).Count(&total)

					uf.Println(aurora.Cyan("Page"), aurora.Bold(page), "of", aurora.Yellow(total/10+1))

					if len(submits) == 0 {
						uf.Println(aurora.Gray(15, "No submissions yet"))
					} else {
						// t.AddHeader("ID", "Problem", "Status", "Message", "Score", "Judge Message")
						Cols := []string{"ID", "Problem", "Status", "Message", "Score", "Judge Message", "Date"}
						var ColLongest = make([]int, len(Cols))
						for i, col := range Cols {
							ColLongest[i] = len(col)
						}

						for _, submit := range submits {
							ColLongest[0] = max(ColLongest[0], len(submit.ID))
							ColLongest[1] = max(ColLongest[1], len(submit.Problem))
							ColLongest[2] = max(ColLongest[2], len(submit.Status))
							ColLongest[3] = max(ColLongest[3], len(submit.Msg))
							ColLongest[4] = max(ColLongest[4], len(fmt.Sprintf("%.2f", submit.JudgeResult.Score)))
							ColLongest[5] = max(ColLongest[5], len(submit.JudgeResult.Msg))
							ColLongest[6] = max(ColLongest[6], len(time.Unix(0, submit.SubmitTime).Format(time.DateTime)))
						}

						for i, col := range Cols {
							uf.Printf("%-*s ", ColLongest[i], col)
						}
						uf.Println()
						for _, submit := range submits {
							// uf.Println(aurora.Magenta(submit.ID), submit.Problem, ColorizeStatus(submit.Status), aurora.Blue(submit.Msg), aurora.Bold(submit.JudgeResult.Score))
							uf.Printf("%-*s %-*s %-*s %-*s %-*.2f %-*s %-*s\n",
								ColLongest[0], aurora.Magenta(submit.ID),
								ColLongest[1], aurora.Bold(aurora.Italic(submit.Problem)),
								ColLongest[2], ColorizeStatus(submit.Status),
								ColLongest[3], aurora.Blue(submit.Msg),
								ColLongest[4], aurora.Bold(ColorizeScore(submit.JudgeResult)),
								ColLongest[5], aurora.Bold(aurora.Blue(submit.JudgeResult.Msg)),
								ColLongest[6], aurora.Yellow(time.Unix(0, submit.SubmitTime).Format(time.DateTime+" MST")))
						}
					}

				case "status":
					if len(cmds) != 2 {
						uf.Println(aurora.Red("error:"), "invalid arguments")
						uf.Println("usage: status <submit_id>")
						return
					}

					uf.Println(aurora.Green("Showing"), aurora.Bold("submission"), aurora.Magenta(cmds[1]))

					var submit SubmitCtx
					tx := db.Where("id = ? AND user = ?", cmds[1], s.User()).First(&submit)
					if tx.Error != nil {
						uf.Println(aurora.Red("error:"), "submit", aurora.Yellow(strconv.Quote(cmds[1])), "not found")
						return
					}

					uf.Println()

					// if submit.Status == "completed" {
					WriteResult(uf, submit)
					// }

					uf.Println("Submit", "is", ColorizeStatus(submit.Status))

					if len(submit.Msg) > 0 {
						uf.Println("Message:\n	", aurora.Blue(submit.Msg))
					} else {
						uf.Println("Message:\n	", aurora.Gray(15, "No message"))
					}

					uf.Println("\nLogs:")

					s.Write(submit.Userface.Buffer.Bytes())

					// c, _ := json.Marshal(submit)
					// fmt.Println(string(c))

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

func WriteResult(uf Userface, res SubmitCtx) {
	if res.Status != "completed" {
		uf.Println(aurora.Italic(aurora.Underline(aurora.Bold(aurora.Gray(15, "No judgement result")))))
		uf.Println()
		return
	}
	if res.JudgeResult.Success {
		uf.Printf("%.2f\n", aurora.Bold(ColorizeScore(res.JudgeResult)))
	} else {
		uf.Println(aurora.Red("Judgement is Failed"))
	}

	uf.Println("Judgement Message:")

	if len(res.JudgeResult.Msg) > 0 {
		uf.Println("	", aurora.Bold(aurora.Cyan(res.JudgeResult.Msg)))
	} else {
		uf.Println("	", aurora.Gray(15, "No message"))
	}
	uf.Println()
}
