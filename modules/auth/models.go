package auth

import "time"

type User struct {
	UserID        uint      `gorm:"primaryKey;autoIncrement" json:"userId"`
	Name          string    `gorm:"size:50;not null" json:"name"`
	Surname       string    `gorm:"size:50;not null" json:"surname"`
	Email         string    `gorm:"size:255;not null;uniqueIndex" json:"email"`
	PasswordHash  string    `gorm:"not null" json:"-"`
	PassChangedAt time.Time `gorm:"not null" json:"passChangedAt"`
	CreatedAt     time.Time `gorm:"not null;autoCreateTime" json:"createdAt"`
	IsActive      bool      `gorm:"not null;default:true" json:"isActive"`
	LastLoginAt   time.Time `gorm:"not null" json:"lastLoginAt"`
	Timezone      string    `gorm:"type:varchar(64);not null;default:'UTC'" json:"timezone"`
}

type RegisterDTO struct {
	Name     string `json:"name" validate:"required,min=2"`
	Surname  string `json:"surname" validate:"required,min=2"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=64"`
	Timezone string `json:"timezone" validate:"required,max=64"`
}

type LoginDTO struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=64"`
}

type MeDTO struct {
	UserID      uint      `json:"userId"`
	Name        string    `json:"name"`
	Surname     string    `json:"surname"`
	Email       string    `json:"email"`
	IsActive    bool      `json:"isActive"`
	LastLoginAt time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Session struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"userId"`
	TokenHash string    `gorm:"not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expiresAt"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
}

type PasswordResetToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"userId"`
	TokenHash string    `gorm:"not null;uniqueIndex;size:255" json:"-"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expiresAt"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime" json:"createdAt"`
}

type ForgotPassDTO struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPassDTO struct {
	Token           string `json:"token" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8,max=64"`
	ConfirmPassword string `json:"confirmPassword" validate:"required,eqfield=NewPassword"`
}
