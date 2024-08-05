package main

import (
	"database/sql/driver"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type User struct {
	ID    string `gorm:"primaryKey"`
	Token string `gorm:"uniqueIndex" json:"-"`

	BestScores     JMapStrFloat64
	BestSubmits    JMapStrString
	BestSubmitDate JMapStrInt64

	TotalScore float64
}

type JMapStrFloat64 map[string]float64
type JMapStrString map[string]string
type JMapStrInt64 map[string]int64

func (u *User) CalculateTotalScore() {
	var total float64
	for _, s := range u.BestScores {
		total += s
	}
	u.TotalScore = total
}

func DoFULLUserScan(pmbls map[string]Problem) {

	var _submits []SubmitCtx
	db.Find(&_submits)

	var _users []User
	db.Find(&_users)

	users := make(map[string]User)
	for _, user := range _users { // Load users into map
		users[user.ID] = user
	}

	for _, s := range _submits {
		u, ok := users[s.User]
		if !ok {
			// User does not exist in map, nor in database, but exists in submits
			log.Fatal().Msg("Encountered corrupted data, submitted user does not exist in User table")
		}

		if s.Status == "completed" {
			if u.BestScores[s.Problem] < s.JudgeResult.Score*pmbls[s.Problem].Weight {
				u.BestScores[s.Problem] = s.JudgeResult.Score * pmbls[s.Problem].Weight
				u.BestSubmits[s.Problem] = s.ID
				u.BestSubmitDate[s.Problem] = s.SubmitTime
			}
		}

		users[s.User] = u
	}

	for _, u := range users {
		u.CalculateTotalScore()
		db.Save(&u)
	}

}

func UserUpdate(user string, s SubmitCtx) {
	var u User
	db.First(&u, "id = ?", user)

	if u.ID == "" { // Create a new user in database
		u.ID = user
		u.Token = uuid.New().String()
		u.BestScores = make(map[string]float64)
		u.BestSubmits = make(map[string]string)
		u.BestSubmitDate = make(map[string]int64)
		log.Info().Str("user", user).Msg("Creating new user")
	}

	if s.Status == "completed" {
		if u.BestScores[s.Problem] < s.JudgeResult.Score*s.problem.Weight {
			u.BestScores[s.Problem] = s.JudgeResult.Score * s.problem.Weight
			u.BestSubmits[s.Problem] = s.ID
			u.BestSubmitDate[s.Problem] = s.SubmitTime
		}
	}

	u.CalculateTotalScore()

	db.Save(&u)
}

func GetToken(user string) string {
	var u User
	db.First(&u, "id = ?", user)

	if u.ID == "" { // Create a new user in database
		u.ID = user
		u.Token = uuid.New().String()
		u.BestScores = make(map[string]float64)
		u.BestSubmits = make(map[string]string)
		u.BestSubmitDate = make(map[string]int64)
		log.Info().Str("user", user).Msg("Creating new user")
		db.Save(&u)
	}

	return u.Token
}

func (sh JMapStrFloat64) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

func (sh *JMapStrFloat64) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal(b, sh)
	}
	return json.Unmarshal(b, sh)
}

func (sh JMapStrString) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

func (sh *JMapStrString) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal(b, sh)
	}
	return json.Unmarshal(b, sh)
}

func (sh JMapStrInt64) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

func (sh *JMapStrInt64) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return json.Unmarshal(b, sh)
	}
	return json.Unmarshal(b, sh)
}
