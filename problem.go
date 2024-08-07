package main

import (
	"log"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Image string   `yaml:"image"`
	Steps []string `yaml:"steps"`

	Timeout         int     `yaml:"timeout"`
	Root            bool    `yaml:"root"`
	DisableNetwork  bool    `yaml:"disablenetwork"`
	Show            []int   `yaml:"show"`
	PrivilegedSteps []int   `yaml:"privilegedsteps"`
	NetworkHostMode bool    `yaml:"networkhostmode"`
	Mounts          []Mount `yaml:"mounts"`
}

type Mount struct {
	Type     string `yaml:"type"`
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"readonly"`
}

type Submit struct {
	Path string `yaml:"path"`
	// MaxSize int
	IsDir bool `yaml:"isdir"`
	// Requred bool
}

type Problem struct {
	Version int `yaml:"version"`

	Id string `yaml:"id"`

	Text string `yaml:"text"`

	Weight float64 `yaml:"weight"`

	Submits []Submit `yaml:"submits"`

	Workflow []Workflow `yaml:"workflow"`
}

func LoadProblem(file string) Problem {
	_f, err := os.ReadFile(file)

	if err != nil {
		panic(err)
	}

	var _p Problem

	err = yaml.Unmarshal(_f, &_p)

	if err != nil {
		panic(errors.Wrap(err, "failed to unmarshal problem "+file))
	}

	if _p.Weight == 0 {
		_p.Weight = 1.0
	}

	pblms = append(pblms, _p.Id)
	return _p

}

func LoadProblemDir(dir string) map[string]Problem {
	_f, err := os.ReadDir(dir)

	if err != nil {
		panic(err)
	}

	var _p = make(map[string]Problem)

	for _, f := range _f {
		var _pf = LoadProblem(dir + "/" + f.Name())
		_p[_pf.Id] = _pf
		log.Println("loaded problem", _pf.Id)
	}

	return _p
}

var pblms []string
