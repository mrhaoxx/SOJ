package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/mount"
	"github.com/google/uuid"
)

type JudgeResult struct {
	Success bool
	Score   int

	Msg string

	Memory uint64 // in bytes
	Time   uint64 // in ns
}

type Workflow struct {
	Image string
	Steps []string

	Timeout int // in ns
}

type Submit struct {
	Path    string
	MaxSize int
	IsDir   bool
	Requred bool
}

type Problem struct {
	Text string

	Submits []Submit

	Workflow []Workflow
}

func SubmitJudge(user string, problem Problem) JudgeResult {

	var _uid, err = uuid.NewV7()
	if err != nil {
		return JudgeResult{
			Success: false,
			Msg:     "failed to generate uuid",
		}
	}

	var _uid_str = _uid.String()

	// create working dir
	var submit_workdir = cfg.SubmitWorkDir + "/" + time.Now().Format("20060102150405") + "-" + user

	log.Println(user, "submit workdir", submit_workdir)

	if err := os.MkdirAll(submit_workdir, 0700); err != nil {
		return JudgeResult{
			Success: false,
			Msg:     "failed to create working dir",
		}
	}

	var submit_dir = submit_workdir + "/submits"

	if err := os.MkdirAll(submit_dir, 0700); err != nil {
		return JudgeResult{
			Success: false,
			Msg:     "failed to create submit dir",
		}
	}

	//copy submits
	for _, submit := range problem.Submits {

		var src_submit_path = path.Join(cfg.SubmitsDir, user, submit.Path)
		var dst_submit_path = path.Join(submit_dir, submit.Path)

		if submit.IsDir {
			if err := CopyDir(src_submit_path, dst_submit_path); err != nil {
				log.Println("failed to copy submit dir", src_submit_path, dst_submit_path, err)
				return JudgeResult{
					Success: false,
					Msg:     "failed to copy submit dir",
				}
			}
		} else {
			if err := CopyFile(src_submit_path, dst_submit_path); err != nil {
				log.Println("failed to copy submit file", src_submit_path, dst_submit_path, err)
				return JudgeResult{
					Success: false,
					Msg:     "failed to copy submit file",
				}
			}
		}

		log.Println("copy submit", src_submit_path, dst_submit_path)
	}

	var work_dir = submit_workdir + "/work"

	if err := os.MkdirAll(work_dir, 0700); err != nil {
		return JudgeResult{
			Success: false,
			Msg:     "failed to create work dir",
		}
	}

	var mount = []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: submit_dir,
			Target: "/submits",
		},
		{
			Type:   mount.TypeBind,
			Source: work_dir,
			Target: "/work",
		},
	}

	for _, workflow := range problem.Workflow {
		ok, cid := RunImage("soj-judge-"+_uid_str, "1000", "soj-judge"+_uid_str, workflow.Image, "/work", mount, false, false)

		if !ok {
			return JudgeResult{
				Success: false,
				Msg:     "failed to run judge container",
			}
		}

		defer CleanContainer(cid)

		for _, step := range workflow.Steps {
			err := ExecContainer(cid, step, workflow.Timeout)

			if err != nil {
				return JudgeResult{
					Success: false,
					Msg:     "failed to exec judge container",
				}
			}
		}

	}

	var result_file = work_dir + "/result.json"

	_result, err := os.ReadFile(result_file)

	if err != nil {
		return JudgeResult{
			Success: false,
			Msg:     "failed to read result file",
		}
	}

	var result = JudgeResult{}

	err = json.Unmarshal(_result, &result)
	if err != nil {
		return JudgeResult{
			Success: false,
			Msg:     "failed to parse result file",
		}
	}

	return result
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	return destinationFile.Sync()
}

// CopyDir copies a whole directory recursively from src to dst.
func CopyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	sInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, sInfo.Mode()); err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
