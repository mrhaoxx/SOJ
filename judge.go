package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog/log"

	"github.com/docker/docker/api/types/mount"
)

type JudgeResult struct {
	Success bool

	Score int

	Msg string

	Memory uint64 // in bytes
	Time   uint64 // in ns

	Speedup float64
}

type WorkflowResult struct {
	Success  bool
	Logs     string
	ExitCode int
}

type userface struct {
	*bytes.Buffer
	io.Writer
}

func (f *userface) Print(a ...interface{}) (n int, err error) {
	return fmt.Fprint(io.MultiWriter(f.Buffer, f.Writer), a...)
}

type SubmitCtx struct {
	ID      string `gorm:"primaryKey"`
	User    string
	Problem string

	problem *Problem

	SubmitTime int64
	LastUpdate int64

	Status string
	Msg    string

	SubmitDir       string
	SubmitsHashes   map[string]string
	Workdir         string
	WorkflowResults []WorkflowResult
	JudgeResult     JudgeResult

	running  chan struct{}
	userface userface
}

func (ctx *SubmitCtx) Update() *SubmitCtx {
	ctx.LastUpdate = time.Now().UnixNano()
	return ctx
}

func (ctx *SubmitCtx) SetStatus(status string) *SubmitCtx {
	ctx.Status = status
	return ctx
}

func (ctx *SubmitCtx) SetMsg(msg string) *SubmitCtx {
	ctx.Msg = msg
	return ctx
}

func RunJudge(ctx *SubmitCtx) {
	log.Debug().Timestamp().Str("id", ctx.ID).Str("user", ctx.User).Str("problem", ctx.Problem).Msg("run judge")

	var err error

	defer func() {
		log.Debug().Timestamp().Str("id", ctx.ID).Str("status", ctx.Status).Str("judgemsg", ctx.Msg).AnErr("err", err).Msg("judge finished")
		close(ctx.running)
	}()

	ctx.userface.Print("Submission", aurora.Magenta(ctx.ID), aurora.Green("running"))

	ctx.SetStatus("prep_dirs").Update()

	var submits_dir = path.Join(ctx.Workdir, "submits")
	var workflow_dir = path.Join(ctx.Workdir, "work")

	err = os.Mkdir(ctx.Workdir, 0700)
	if err != nil {
		goto workdir_creation_failed
	}
	err = os.Mkdir(submits_dir, 0700)
	if err != nil {
		goto workdir_creation_failed
	}
	err = os.Mkdir(workflow_dir, 0700)
	if err != nil {
		goto workdir_creation_failed
	}

	goto workdir_created

workdir_creation_failed:

	ctx.SetStatus("failed").SetMsg("failed to create submit workdir").Update()
	return

workdir_created:

	log.Debug().Timestamp().Str("id", ctx.ID).Str("submit_workdir", ctx.Workdir).Msg("created working dirs")

	ctx.SetStatus("prep_files").Update()

	for _, submit := range ctx.problem.Submits {

		var src_submit_path = path.Join(ctx.SubmitDir, submit.Path)
		var dst_submit_path = path.Join(submits_dir, submit.Path)

		var hash string
		hash, err = CopyFile(src_submit_path, dst_submit_path)
		if err != nil {
			ctx.SetStatus("failed").SetMsg("failed to copy submit file " + strconv.Quote(submit.Path)).Update()
		} else {
			log.Debug().Timestamp().Str("id", ctx.ID).Str("submit_file", submit.Path).Str("hash", hash).Msg("copied submit file")
			ctx.SubmitsHashes[submit.Path] = hash
		}

	}

	log.Debug().Timestamp().Str("id", ctx.ID).Msg("copied submit files")

	ctx.SetStatus("run_workflow").Update()

	var mount = []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: submits_dir,
			Target: "/submits",
		},
		{
			Type:   mount.TypeBind,
			Source: workflow_dir,
			Target: "/work",
		},
	}

	for idx, workflow := range ctx.problem.Workflow {

		ctx.SetStatus("run_workflow-" + strconv.Itoa(idx)).Update()

		ok, cid := RunImage("soj-judge-"+ctx.ID, "1000", "soj-judgement", workflow.Image, "/work", mount, false, false)

		if !ok {
			ctx.SetStatus("failed").SetMsg("failed to run judge container").Update()
			return
		}

		defer CleanContainer(cid)

		for sidx, step := range workflow.Steps {
			ctx.SetStatus("run_workflow-" + strconv.Itoa(idx) + "_" + strconv.Itoa(sidx)).Update()

			ec, logs, err := ExecContainer(cid, step, workflow.Timeout)

			if ec != 0 || err != nil {
				ctx.SetStatus("failed").SetMsg("failed to run judge step").Update()

				log.Info().Timestamp().Str("id", ctx.ID).Str("image", workflow.Image).Str("step", step).Int("timeout", workflow.Timeout).AnErr("err", err).Str("logs", logs).Int("exitcode", ec).Msg("failed to run judge step")
				return
			}

			ctx.WorkflowResults = append(ctx.WorkflowResults, WorkflowResult{
				Success:  true,
				Logs:     logs,
				ExitCode: ec,
			})
		}
	}

	ctx.SetStatus("collect_result").Update()

	var result_file = workflow_dir + "/result.json"

	_result, err := os.ReadFile(result_file)

	if err != nil {
		log.Info().Timestamp().Str("id", ctx.ID).Str("result_file", result_file).AnErr("err", err).Msg("failed to read result file")
		ctx.SetStatus("failed").SetMsg("failed to read result file").Update()
	}

	err = json.Unmarshal(_result, &ctx.JudgeResult)
	if err != nil {
		log.Info().Timestamp().Str("id", ctx.ID).Str("result_file", result_file).AnErr("err", err).Msg("failed to parse result file")
		ctx.SetStatus("failed").SetMsg("failed to parse result file").Update()
	}

	ctx.SetStatus("completed").SetMsg("judge successfully finished").Update()
}

// CopyFile copies a single file from src to dst and returns the MD5 hash of the copied file.
func CopyFile(src, dst string) (string, error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer destinationFile.Close()

	hash := md5.New()
	if _, err = io.Copy(destinationFile, io.TeeReader(sourceFile, hash)); err != nil {
		return "", err
	}

	if err := destinationFile.Sync(); err != nil {
		return "", err
	}

	// Calculate the MD5 sum of the file that has been copied.
	md5String := hex.EncodeToString(hash.Sum(nil))
	return md5String, nil
}
