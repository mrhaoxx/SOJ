package database

import (
	"github.com/rs/zerolog/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"sync"
)

var (
	db   *gorm.DB
	once sync.Once
)

func InitDB(sqlitePath string) *gorm.DB {
	once.Do(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
		if err != nil {
			log.Fatal().Err(err).Msg("failed to open sqlite")
		}
	})
	return db
}

func GetDB() *gorm.DB {
	once.Do(func() {
		log.Fatal().Msg("Used database before initialization")
	})
	return db
}
