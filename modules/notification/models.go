package notification

import "time"

type DeviceToken struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint      `gorm:"not null;index" json:"userId"`
	SessionID  string    `gorm:"not null;index" json:"sessionId"`
	Token      string    `gorm:"not null;uniqueIndex" json:"token"`
	Platform   string    `gorm:"not null;size:20" json:"platform"` // ios, android
	DeviceID   *string   `gorm:"size:191;index" json:"deviceId,omitempty"`
	AppVersion *string   `gorm:"size:50" json:"appVersion,omitempty"`
	IsActive   bool      `gorm:"not null;default:true;index" json:"isActive"`
	LastSeenAt time.Time `gorm:"not null;index" json:"lastSeenAt"`
	CreatedAt  time.Time `gorm:"not null" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"not null" json:"updatedAt"`
}

type RegisterPushTokenDTO struct {
	Token      string  `json:"token" validate:"required,min=20"`
	Platform   string  `json:"platform" validate:"required,oneof=ios android web"`
	DeviceID   *string `json:"deviceId,omitempty"`
	AppVersion *string `json:"appVersion,omitempty"`
}

type DeletePushTokenDTO struct {
	Token string `json:"token" validate:"required,min=20"`
}
