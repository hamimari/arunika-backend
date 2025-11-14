package models

type UserActivities struct {
	BaseModel
	UserId string `json:"user_id"`
	GameId string `json:"game_id"`
}
