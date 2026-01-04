package models

type Feature struct {
	BaseModel
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string
	IsActive    bool
}
