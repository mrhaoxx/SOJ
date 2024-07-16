package main

import (
	"gorm.io/gorm"
)

var db *gorm.DB

type SubmitRecord struct {
	ID      string `gorm:"primaryKey"`
	User    string
	Problem string

	SubmitTime int64
	LastUpdate int64

	Status string
	Msg    string

	WorkflowStep string

	SubmitsHash string

	WorkflowLogs []string
}
