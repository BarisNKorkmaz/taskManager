package database

import (
	"time"

	"gorm.io/gorm"
)

func Create(value any) *gorm.DB {
	return DB.Create(value)
}

func FetchUserByEmail(email string, dest any) *gorm.DB {
	return DB.Where("email = ?", email).First(dest)
}

func FetchUserByUID(userID uint, dest any) *gorm.DB {
	return DB.Where("userID = ?", userID).First(dest)
}

func UpdateLastLogin(model any, query string, args ...any) *gorm.DB {
	return DB.Model(model).Where(query, args...).Update("last_login_at", time.Now())
}
