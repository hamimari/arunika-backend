package models

import "gorm.io/gorm"

type Resources struct {
	gorm.Model
	GameId      string `json:"game_id"`
	Name        string `json:"name"`
	ResourceUrl string `json:"resource_url"`
}
