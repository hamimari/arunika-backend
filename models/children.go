package models

import "time"

type Children struct {
	BaseModel
	Name        string    `json:"name"`
	Gender      string    `json:"gender"`
	DateOfBirth time.Time `json:"date_of_birth"`
	ParentId    string    `json:"parent_id"`
	StarPoints  int       `json:"star_points"`
}

func (Children) TableName() string {
	return "children"
}
