package main

import (
	"fmt"
	"github.com/joker782311/cryptoArbitrage/internal/config"
	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/pkg/logger"
)

func main() {
	cfg := config.Load()

	if err := logger.Init(); err != nil {
		panic(err)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.DBName)

	if err := database.InitMySQL(dsn); err != nil {
		panic(err)
	}

	if err := database.InitRedis(cfg.Redis.Host+":"+cfg.Redis.Port, cfg.Redis.Password); err != nil {
		panic(err)
	}

	if err := database.Migrate(); err != nil {
		panic(err)
	}

	logger.Log.Info("Server starting on port " + cfg.Server.Port)
}
