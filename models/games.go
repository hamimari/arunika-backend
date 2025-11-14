package models

type Games struct {
	BaseModel
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Instruction string  `json:"instruction"`
	Point       float64 `json:"point"`
	AgeStart    float32 `json:"age_start"`
	AgeEnd      float32 `json:"age_end"`
	CategoryId  string  `json:"category_id"`
}
