package main

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"

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

// Value 实现了 driver.Valuer 接口，使得 SubmitHash 可以被自动序列化为 JSON 字符串
func (sh SubmitHash) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

// Scan 实现了 sql.Scanner 接口，使得 JSON 字符串可以被自动反序列化为 SubmitHash
func (sh *SubmitHash) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal(b, sh)
	}
	return json.Unmarshal(b, sh)
}

// SubmitsHashes 代表多个 SubmitHash
type SubmitsHashes []SubmitHash

// Value 实现了 driver.Valuer 接口
func (sh SubmitsHashes) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

// Scan 实现了 sql.Scanner 接口
func (sh *SubmitsHashes) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal(b, sh)
	}
	return json.Unmarshal(b, sh)
}

// Value 实现了 driver.Valuer 接口，使得 SubmitHash 可以被自动序列化为 JSON 字符串
func (sh WorkflowResult) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

// Scan 实现了 sql.Scanner 接口，使得 JSON 字符串可以被自动反序列化为 SubmitHash
func (sh *WorkflowResult) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal(b, sh)
	}
	return json.Unmarshal(b, sh)
}

// SubmitsHashes 代表多个 SubmitHash
type WorkflowResults []WorkflowResult

// Value 实现了 driver.Valuer 接口
func (sh WorkflowResults) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

// Scan 实现了 sql.Scanner 接口
func (sh *WorkflowResults) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal(b, sh)
	}
	return json.Unmarshal(b, sh)
}

// Value 实现了 driver.Valuer 接口，使得 SubmitHash 可以被自动序列化为 JSON 字符串
func (sh JudgeResult) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

// Scan 实现了 sql.Scanner 接口，使得 JSON 字符串可以被自动反序列化为 SubmitHash
func (sh *JudgeResult) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal(b, sh)
	}
	return json.Unmarshal(b, sh)
}

// Value 实现了 driver.Valuer 接口，使得 SubmitHash 可以被自动序列化为 JSON 字符串
func (sh Userface) Value() (driver.Value, error) {
	return sh.Buffer.String(), nil
}

// Scan 实现了 sql.Scanner 接口，使得 JSON 字符串可以被自动反序列化为 SubmitHash
func (sh *Userface) Scan(value interface{}) error {
	b, ok := value.(string)
	if !ok {
		return errors.New("type assertion to string failed")
	}
	sh.Buffer = bytes.NewBufferString(b)
	return nil
}
