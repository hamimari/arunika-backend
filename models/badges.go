package models

type Badges struct {
	BaseModel
	Name     string `json:"name"`
	ImageUrl string `json:"image_url"`
}
