package database

import "github.com/joker782311/cryptoArbitrage/internal/model"

func Migrate() error {
	return DB.AutoMigrate(
		&model.User{},
		&model.APIKey{},
		&model.Strategy{},
		&model.Order{},
		&model.Position{},
		&model.Alert{},
		&model.AlertConfig{},
		&model.Ticker{},
		&model.FundingRate{},
	)
}
