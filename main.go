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

	DockerCli        string `yaml:"DockerCli"`
	ProblemURLPrefix string `yaml:"ProblemURLPrefix"`
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
				uf.Println("Use 'submit", aurora.Gray(15, "(sub)"), "<problem_id>' to submit a problem")
				uf.Println("Use 'history", aurora.Gray(15, "(hi)"), "[page]' to list your submissions")
				uf.Println("Use 'status", aurora.Gray(15, "(st)"), "<submit_id>' to show a submission")
				uf.Println()

			} else {
				uf.Println(aurora.Yellow(time.Now().Format(time.DateTime + " MST")))

				switch cmds[0] {
				case "ls":
					uf.Println("Problems:", aurora.Italic(aurora.Gray(15, "(click to show in browser)")))
					for k := range problems {
						url := cfg.ProblemURLPrefix + k
						uf.Println("	", aurora.Bold(k), aurora.Hyperlink(url, url))
					}

				case "submit", "sub":
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

				case "history", "hi":
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

					ListSubs(uf, submits)

				case "status", "st":
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

					ShowSub(uf, submit, problems)

				case "my":
					uf.Println("User", aurora.Bold(aurora.BrightWhite(s.User())))

					var map_score = make(map[string]SubmitCtx)

					var submits []SubmitCtx

					db.Where("user = ?", s.User()).Find(&submits)

					for _, submit := range submits {
						if submit.Status == "completed" {
							sc, ok := map_score[submit.Problem]
							if !ok {
								map_score[submit.Problem] = submit
							} else {
								if submit.JudgeResult.Score > sc.JudgeResult.Score {
									map_score[submit.Problem] = submit
								}
							}
						}
					}

					for k := range problems {
						if _, ok := map_score[k]; !ok {
							map_score[k] = SubmitCtx{
								Problem: k,
							}
						}
					}

					var total_score float64

					Cols := []string{"Problem", "Score", "Weight", "Submit ID", "Date"}
					var ColLongest = make([]int, len(Cols))
					for i, col := range Cols {
						ColLongest[i] = len(col)
					}

					for k, submit := range map_score {
						ColLongest[0] = max(ColLongest[0], len(k))
						ColLongest[1] = max(ColLongest[1], len(fmt.Sprintf("%.2f", submit.JudgeResult.Score)))
						ColLongest[2] = max(ColLongest[2], len(fmt.Sprintf("%.2f", problems[k].Weight)))
						ColLongest[3] = max(ColLongest[3], len(submit.ID))
						ColLongest[4] = max(ColLongest[4], len(time.Unix(0, submit.SubmitTime).Format(time.DateTime+" MST")))

						total_score += submit.JudgeResult.Score * problems[k].Weight
					}

					for i, col := range Cols {
						uf.Printf("%-*s ", ColLongest[i], col)
					}

					uf.Println()
					for _, submit := range map_score {
						uf.Printf("%-*s %-*.2f %-*.2f %-*s %-*s\n",
							ColLongest[0], aurora.Bold(aurora.Italic(submit.Problem)),
							ColLongest[1], aurora.Bold(ColorizeScore(submit.JudgeResult)),
							ColLongest[2], aurora.Bold(problems[submit.Problem].Weight),
							ColLongest[3], aurora.Magenta(submit.ID),
							ColLongest[4],
							func() aurora.Value {
								if submit.Status == "completed" {
									return aurora.Yellow(time.Unix(0, submit.SubmitTime).Format(time.DateTime + " MST"))
								} else {
									return aurora.Gray(15, "N/A")
								}
							}())
					}

					uf.Println()
					uf.Println("Total Score:", aurora.Bold(aurora.BrightWhite(total_score)))

				case "adm":
					if len(cmds) < 2 {
						uf.Println(aurora.Red("error:"), "invalid arguments")
						uf.Println("usage: adm <command>")
						return
					}
					switch cmds[1] {
					case "list":
						page := 1
						if len(cmds) == 3 {
							var err error
							page, err = strconv.Atoi(cmds[2])
							if err != nil {
								uf.Println(aurora.Red("error:"), "invalid page number")
								return
							}
						}

						var submits []SubmitCtx
						//paging
						// reverse order

						// db.Where("user = ?", s.User()).Offset((page - 1) * 10).Limit(10).Find(&submits)
						db.Order("submit_time desc").Offset((page - 1) * 10).Limit(20).Find(&submits)

						var total int64
						db.Model(&SubmitCtx{}).Where("user = ?", s.User()).Count(&total)

						uf.Println(aurora.Cyan("Page"), aurora.Bold(page), "of", aurora.Yellow(total/10+1))

						ListSubs(uf, submits)
					case "status":
						if len(cmds) != 3 {
							uf.Println(aurora.Red("error:"), "invalid arguments")
							uf.Println("usage: adm status <submit_id>")
							return
						}

						uf.Println(aurora.Green("Showing"), aurora.Bold("submission"), aurora.Magenta(cmds[2]))

						var submit SubmitCtx
						tx := db.Where("id = ?", cmds[2]).First(&submit)
						if tx.Error != nil {
							uf.Println(aurora.Red("error:"), "submit", aurora.Yellow(strconv.Quote(cmds[2])), "not found")
							return
						}

						uf.Println()

						ShowSub(uf, submit, problems)
					}

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
		uf.Printf("Score %.2f %s\n", aurora.Underline(aurora.Bold(ColorizeScore(res.JudgeResult))), aurora.Italic(aurora.Gray(15, "max.100 (Unweighted)")))
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

func ListSubs(uf Userface, submits []SubmitCtx) {

	if len(submits) == 0 {
		uf.Println(aurora.Gray(15, "No submissions yet"))
	} else {
		// t.AddHeader("ID", "Problem", "Status", "Message", "Score", "Judge Message")
		Cols := []string{"ID", "Problem", "Status", "Message", "Score", "Judge Message", "User", "Date"}
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
			ColLongest[6] = max(ColLongest[6], len(submit.User))
			ColLongest[7] = max(ColLongest[7], len(time.Unix(0, submit.SubmitTime).Format(time.DateTime)))
		}

		for i, col := range Cols {
			uf.Printf("%-*s ", ColLongest[i], col)
		}
		uf.Println()
		for _, submit := range submits {
			// uf.Println(aurora.Magenta(submit.ID), submit.Problem, ColorizeStatus(submit.Status), aurora.Blue(submit.Msg), aurora.Bold(submit.JudgeResult.Score))
			uf.Printf("%-*s %-*s %-*s %-*s %-*.2f %-*s %-*s %-*s\n",
				ColLongest[0], aurora.Magenta(submit.ID),
				ColLongest[1], aurora.Bold(aurora.Italic(submit.Problem)),
				ColLongest[2], ColorizeStatus(submit.Status),
				ColLongest[3], aurora.Blue(submit.Msg),
				ColLongest[4], aurora.Bold(ColorizeScore(submit.JudgeResult)),
				ColLongest[5], aurora.Bold(aurora.Blue(submit.JudgeResult.Msg)),
				ColLongest[6], aurora.Bold(aurora.BrightWhite(submit.User)),
				ColLongest[7], aurora.Yellow(time.Unix(0, submit.SubmitTime).Format(time.DateTime+" MST")))
		}
	}
}

func ShowSub(uf Userface, submit SubmitCtx, problems map[string]Problem) {
	uf.Println("Submit", "Status:", ColorizeStatus(submit.Status))
	uf.Println("Submit Time:", aurora.Yellow(time.Unix(0, submit.SubmitTime).Format(time.DateTime+" MST")))
	uf.Println("Last Update:", aurora.Yellow(time.Unix(0, submit.LastUpdate).Format(time.DateTime+" MST")))
	uf.Println("User", aurora.Bold(aurora.BrightWhite(submit.User)))

	if len(submit.Msg) > 0 {
		uf.Println("Message:\n	", aurora.Blue(submit.Msg))
	} else {
		uf.Println("Message:\n	", aurora.Gray(15, "No message"))
	}

	uf.Println()

	if prob, ok := problems[submit.Problem]; ok {
		uf.Println("Problem:", aurora.Bold(submit.Problem))
		uf.Println("Problem Weight:", aurora.Bold(prob.Weight))
	} else {
		uf.Println("Problem:", aurora.Bold(submit.Problem), aurora.Gray(15, "(not found)"))
	}

	uf.Println()

	uf.Println("Logs:")
	uf.Write(submit.Userface.Buffer.Bytes())

	uf.Println()

	WriteResult(uf, submit)

	// c, _ := json.Marshal(submit)
	// fmt.Println(string(c))
}
