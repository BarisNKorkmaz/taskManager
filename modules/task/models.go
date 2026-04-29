package task

import "time"

type CategoryType string

const (
	Work     CategoryType = "work"
	Personal CategoryType = "personal"
	Health   CategoryType = "health"
	Finance  CategoryType = "finance"
	Learning CategoryType = "learning"
	Home     CategoryType = "home"
	Social   CategoryType = "social"
	Other    CategoryType = "other"
)

type TaskTemplate struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint   `gorm:"not null;index" json:"userId"`
	Title       string `gorm:"size:120;not null" json:"title"`
	Description string `gorm:"size:255" json:"description,omitempty"`

	//IsRepeatEnabled bool   `gorm:"not null;default:false" json:"isRepeatEnabled"`
	RepeatType string `gorm:"not null" json:"repeatType"` // once / weekly / interval

	IsActive bool         `gorm:"not null;default:true;index" json:"isActive"`
	Category CategoryType `gorm:"type:varchar(20);column:category;default:'other';not null;check:category IN ('work', 'personal', 'health', 'finance', 'learning', 'home', 'social', 'other')" json:"category"`

	// if RepeatType once
	DueDate *time.Time `gorm:"type:date" json:"dueDate,omitempty"`

	// if RepeatType interval
	RepeatUnit     *string    `gorm:"size:10" json:"repeatUnit,omitempty"` // day | week | month
	RepeatInterval *int       `gorm:"check:repeat_interval > 0" json:"repeatInterval,omitempty"`
	StartDate      *time.Time `gorm:"type:date" json:"startDate,omitempty"`

	// if RepeatType weekly
	WeekDays *string `gorm:"size:20" json:"weekDays,omitempty"` // "1,3,6"

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TaskOccurrence struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID      uint       `gorm:"not null;uniqueIndex:ux_task_due;index" json:"taskId"`
	UserID      uint       `gorm:"not null;index:idx_occ_user_due;index:idx_occ_user_status_due" json:"userId"`
	DueDate     time.Time  `gorm:"not null;type:date;uniqueIndex:ux_task_due;index:idx_occ_user_due;index:idx_occ_user_status_due" json:"dueDate"`
	Status      string     `gorm:"type:varchar(20);not null;default:'pending';index:idx_occ_user_status_due" json:"status"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type TaskDTO struct {
	Title       string `json:"title" validate:"required,min=2,max=120"`
	Description string `json:"description" validate:"omitempty,max=500"`
	Category    string `json:"category" validate:"oneof=work personal health finance learning home social other"`

	RepeatType string `json:"repeatType" validate:"omitempty,oneof=once weekly interval"`

	// once
	DueDate *time.Time `json:"dueDate" validate:"omitempty"`

	// interval
	RepeatUnit     *string    `json:"repeatUnit" validate:"omitempty,oneof=day week month"`
	RepeatInterval *int       `json:"repeatInterval" validate:"omitempty,min=1"`
	StartDate      *time.Time `json:"startDate" validate:"omitempty"`

	//weekly
	WeekDays *string `json:"weekDay" validate:"omitempty"`
}

type TaskActionDTO struct {
	Action     string     `json:"action" validate:"required,oneof=complete undo skip reschedule"`
	NewDueDate *time.Time `json:"newDueDate,omitempty"`
}

type UpdateTemplateDTO struct {
	Title       *string `json:"title" validate:"omitempty,min=1,max=120"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=255"`
	Category    *string `json:"category,omitempty" validate:"omitempty,oneof=work personal health finance learning home social other"`

	DueDate *time.Time `json:"dueDate,omitempty" validate:"omitempty"`

	RepeatUnit     *string    `json:"repeatUnit,omitempty" validate:"omitempty,oneof=day week month"`
	RepeatInterval *int       `json:"repeatInterval,omitempty" validate:"omitempty,gt=0"`
	StartDate      *time.Time `json:"startDate,omitempty" validate:"omitempty"`
}

type SetTemplateStatusDTO struct {
	IsActive *bool `json:"isActive" validate:"required"`
}

type DashboardOccurrenceDTO struct {
	ID          uint         `json:"id"`
	TaskID      uint         `json:"taskId"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Category    CategoryType `json:"category"`
	DueDate     time.Time    `json:"dueDate"`
	Status      string       `json:"status"`
}

type ReportOccurrenceCompletedDTO struct {
	ID          uint         `json:"id"`
	TaskID      uint         `json:"taskId"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Category    CategoryType `json:"category"`
	DueDate     time.Time    `json:"dueDate"`
	CompletedAt time.Time    `json:"completedAt"`
}

type ReportOccurrenceLateCompletedDTO struct {
	ID          uint         `json:"id"`
	TaskID      uint         `json:"taskId"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Category    CategoryType `json:"category"`
	DueDate     time.Time    `json:"dueDate"`
	CompletedAt time.Time    `json:"completedAt"`
	Delay       int          `json:"delay"`
}

type ReportOccurrenceSkippedDTO struct {
	ID          uint         `json:"id"`
	TaskID      uint         `json:"taskId"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Category    CategoryType `json:"category"`
	DueDate     time.Time    `json:"dueDate"`
}
