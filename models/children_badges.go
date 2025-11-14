package models

type ChildrenBadges struct {
	BaseModel
	ChildId  string `json:"child_id"`
	BadgesId string `json:"badges_id"`
}
