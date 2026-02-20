package auth

import "time"

type User struct {
	UserID       uint      `gorm:"primaryKey;autoIncrement" json:"userId"`
	Name         string    `gorm:"size:50;not null" json:"name"`
	Surname      string    `gorm:"size:50;not null" json:"surname"`
	Email        string    `gorm:"size:255;not null;uniqueIndex" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	CreatedAt    time.Time `gorm:"not null;autoCreateTime" json:"createdAt"`
	IsActive     bool      `gorm:"not null;default:true" json:"isActive"`
	LastLoginAt  time.Time `gorm:"not null" json:"lastLoginAt"`
}

type RegisterDTO struct {
	Name     string `json:"name" validate:"required,min=2"`
	Surname  string `json:"surname" validate:"required,min=2"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=64"`
}

type LoginDTO struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=64"`
}
