package task

import "time"

type TaskTemplate struct {
	ID              uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          uint   `gorm:"not null;index" json:"userId"`
	Title           string `gorm:"size:120;not null" json:"title"`
	Description     string `gorm:"size:255" json:"description,omitempty"`
	IsRepeatEnabled bool   `gorm:"not null;default:false" json:"isRepeatEnabled"`
	IsActive        bool   `gorm:"not null;default:true;index" json:"isActive"`

	// if IsRepeatEnabled false
	DueDate *time.Time `gorm:"type:date" json:"dueDate,omitempty"`

	// if IsRepeatEnabled true
	RepeatUnit     *string    `gorm:"size:10" json:"repeatUnit,omitempty"` // day | week | month
	RepeatInterval *int       `gorm:"check:repeat_interval > 0" json:"repeatInterval,omitempty"`
	StartDate      *time.Time `gorm:"type:date" json:"startDate,omitempty"`

	CreatedAt time.Time `json:"createdAt"` // TODO auto inc.
	UpdatedAt time.Time `json:"updatedAt"` // TODO auto inc.
}

type TaskOccurrence struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID      uint       `gorm:"not null;uniqueIndex:ux_task_due;index" json:"taskId"`
	UserID      uint       `gorm:"not null;index:idx_occ_user_due;index:idx_occ_user_completed_due" json:"userId"`
	DueDate     time.Time  `gorm:"not null;type:date;uniqueIndex:ux_task_due;index:idx_occ_user_due;index:idx_occ_user_completed_due" json:"dueDate"`
	IsCompleted bool       `gorm:"not null;default:false;index:idx_occ_user_completed_due" json:"isCompleted"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"` // TODO auto inc.
	UpdatedAt   time.Time  `json:"updatedAt"` // TODO auto inc.
}

type TaskDTO struct {
	Title       string `json:"title" validate:"required,min=2,max=120"`
	Description string `json:"description" validate:"omitempty,max=500"`

	IsRepeatEnabled bool `json:"isRepeatEnabled"`

	// One-time
	DueDate *time.Time `json:"dueDate" validate:"omitempty"`

	// Recurring
	RepeatUnit     *string    `json:"repeatUnit" validate:"omitempty,oneof=day week month"`
	RepeatInterval *int       `json:"repeatInterval" validate:"omitempty,min=1"`
	StartDate      *time.Time `json:"startDate" validate:"omitempty"`
}

type TaskActionDTO struct {
	Action     string     `json:"action" validate:"required,oneof=complete undo skip reschedule"`
	NewDueDate *time.Time `json:"newDueDate,omitempty"`
}

type UpdateTemplateDTO struct {
	Title       *string `json:"title" validate:"omitempty,min=1,max=120"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=255"`

	// Eğer gönderilirse geçerli bir tarih olmalı
	DueDate *time.Time `json:"dueDate,omitempty" validate:"omitempty"`

	// Tekrar ayarları gönderilirse kurallara uymalı
	RepeatUnit     *string    `json:"repeatUnit,omitempty" validate:"omitempty,oneof=day week month"`
	RepeatInterval *int       `json:"repeatInterval,omitempty" validate:"omitempty,gt=0"`
	StartDate      *time.Time `json:"startDate,omitempty" validate:"omitempty"`
}

type SetTemplateStatusDTO struct {
	IsActive *bool `json:"isActive" validate:"required"`
}

type DashboardOccurrenceDTO struct {
	ID          uint      `json:"id"`
	TaskID      uint      `json:"taskId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DueDate     time.Time `json:"dueDate"`
	IsCompleted bool      `json:"isCompleted"`
}
