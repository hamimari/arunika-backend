package models

type Dongeng struct {
	BaseModel
	Title      string  `json:"title"`
	AgeStart   float32 `json:"age_start"`
	AgeEnd     float32 `json:"age_end"`
	ImageUrl   string  `json:"image_url"`
	IsFree     bool    `json:"is_free"`
	CategoryId string  `json:"category_id"`
}
