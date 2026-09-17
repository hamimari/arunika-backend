package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	CampaignStatusSending   = "SENDING"
	CampaignStatusCompleted = "COMPLETED"
	CampaignStatusFailed    = "FAILED"

	CampaignSegmentAllDevices  = "all_devices" // FCM topic — every install, guests included
	CampaignSegmentAll         = "all"         // every registered user
	CampaignSegmentSubscribers = "subscribers" // premium subscribers only

	CampaignLinkNone    = "none"
	CampaignLinkArCard  = "ar_card"
	CampaignLinkDongeng = "dongeng"
)

// Campaign is a push/email blast sent from the backoffice (see V45).
type Campaign struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Title       string     `gorm:"column:title;not null"                          json:"title"`
	Body        string     `gorm:"column:body;not null"                           json:"body"`
	ImageURL    string     `gorm:"column:image_url;not null;default:''"           json:"image_url"`
	Channel     string     `gorm:"column:channel;not null"                        json:"channel"`
	Segment     string     `gorm:"column:segment;not null"                        json:"segment"`
	LinkType    string     `gorm:"column:link_type;not null;default:none"         json:"link_type"`
	LinkID      string     `gorm:"column:link_id;not null;default:''"             json:"link_id"`
	Status      string     `gorm:"column:status;not null;default:SENDING"         json:"status"`
	Sent        int        `gorm:"column:sent;not null;default:0"                 json:"sent"`
	Failed      int        `gorm:"column:failed;not null;default:0"               json:"failed"`
	Error       string     `gorm:"column:error;not null;default:''"               json:"error"`
	CreatedBy   *uuid.UUID `gorm:"column:created_by;type:uuid"                    json:"created_by,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at"                              json:"created_at"`
	CompletedAt *time.Time `gorm:"column:completed_at"                            json:"completed_at,omitempty"`
}

func (Campaign) TableName() string { return "campaigns" }
