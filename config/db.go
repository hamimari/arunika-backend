package config

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
	"time"
)

var (
	DB   *gorm.DB
	RDB  *redis.Client
	once sync.Once
)

func Init() {
	once.Do(func() {
		initDB()
		initRedis()
	})
}

func initDB() {
	dbConfig := LoadDBConfig()

	db, err := gorm.Open(postgres.Open(dbConfig.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get sql.DB: %v", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	DB = db
}

func initRedis() {
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}

	RDB = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), port),
		Password: os.Getenv("REDIS_PASSWORD"), // "" by default
		DB:       0,
	})

	if err := RDB.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
}
