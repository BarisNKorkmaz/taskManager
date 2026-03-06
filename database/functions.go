package database

import (
	"time"

	"gorm.io/gorm"
)

func Create(database *gorm.DB, value any, model any) *gorm.DB {
	return database.Model(model).Create(value)
}

func FetchUserByEmail(email string, dest any) *gorm.DB {
	return DB.Where("email = ?", email).First(dest)
}

func FetchUserByUID(userID uint, dest any) *gorm.DB {
	return DB.Where("user_id = ?", userID).First(dest)
}

func UpdateLastLogin(model any, query string, args ...any) *gorm.DB {
	return DB.Model(model).Where(query, args...).Update("last_login_at", time.Now())
}

func FetchTasksByUID(userId uint, model any, dest any) *gorm.DB {
	return DB.Model(model).Where("user_id = ? AND is_active = ? AND is_repeat_enabled = ?", userId, true, true).Find(dest)
}

func FetchOccurenceByUID(userId uint, model any, dest any, now time.Time, finalDate time.Time) *gorm.DB {
	return DB.Model(model).Select("task_id, due_date").Where("user_id = ? AND due_date BETWEEN ? AND ?", userId, now, finalDate).Find(dest)
}

func FetchUncompletedOccurrences(userId uint, model any, dest any, finalDate time.Time) *gorm.DB {
	return DB.Model(model).Where("user_id = ? AND is_completed = ? AND due_date <= ?", userId, false, finalDate).Order("due_date ASC").Find(dest)
}

func CreateOccurrencesBatch(database *gorm.DB, occs any, model any, batchSize int) *gorm.DB {
	return database.Model(model).CreateInBatches(occs, batchSize)
}

func FetchOccurenceByOccId(model any, occId any, userId uint, dest any) *gorm.DB {
	return DB.Model(model).Where("id = ? AND user_id = ?", occId, userId).First(dest)
}

func UpdateOccStatus(model any, occId any, value any) *gorm.DB {
	return DB.Model(model).Where("id = ?", occId).Select("is_completed", "completed_at", "due_date").Updates(value)
}
