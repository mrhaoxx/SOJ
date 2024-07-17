package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type JudgeResult struct {
	Success bool

	Score int

	Msg string

	Memory uint64 // in bytes
	Time   uint64 // in ns

	Speedup float64
}

func main() {

	//get first file path and read a number from it
	f1 := os.Args[1] // input
	f2 := os.Args[2] //output

	//read file 1
	f1f, err := os.ReadFile(f1)
	if err != nil {
		panic(err)
	}

	//read file 2
	f2f, err := os.ReadFile(f2)
	if err != nil {
		panic(err)
	}

	number, _ := strconv.Atoi(string(strings.Trim(string(f1f), " \n")))

	nans, _ := strconv.Atoi(string(strings.Trim(string(f2f), " \n")))

	fmt.Println(f1, number, f2, nans)

	if number*2 == nans {
		byes, _ := json.Marshal(JudgeResult{
			Success: true,
			Score:   100,
			Msg:     "Correct",
		})
		os.WriteFile("/work/result.json", byes, 0644)
	} else {
		byes, _ := json.Marshal(JudgeResult{
			Success: false,
			Score:   0,
			Msg:     "Incorrect",
		})
		os.WriteFile("/work/result.json", byes, 0644)
	}

}
