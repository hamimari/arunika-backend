package models

type Children struct {
	BaseModel
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	DateOfBirth string `json:"date_of_birth"`
	ParentId    string `json:"parent_id"`
}

func (Children) TableName() string {
	return "children"
}
