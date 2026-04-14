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

func FetchTaskTemplateById(model any, id any, userId uint, dest any) *gorm.DB {
	return DB.Model(model).Where("id = ? AND user_id = ?", id, userId).First(dest)
}

func UpdateTaskTemplate(database *gorm.DB, model any, taskId any, userId uint, value map[string]any) *gorm.DB {
	return database.Model(model).Where("id = ? AND user_id = ?", taskId, userId).Updates(value)
}

func DeleteChangedOccs(database *gorm.DB, model any, taskId any, now time.Time, userId uint) *gorm.DB {
	return database.Model(model).Where("task_id = ? AND user_id = ? AND due_date >= ? AND is_completed = ?", taskId, userId, now, false).Delete(model)
}

func FetchTaskTemplates(model any, userId uint, dest any) *gorm.DB {
	return DB.Model(model).Where("user_id = ?", userId).Order("created_at DESC").Find(dest)
}

func DeleteSessionByUserId(database *gorm.DB, userId uint, model any) *gorm.DB {
	return database.Model(model).Where("user_id = ?", userId).Updates(map[string]interface{}{
		"is_active": false,
	})
}

func FetchSessionByUserId(userId uint, model any, dest any) *gorm.DB {
	return DB.Model(model).Where("user_id = ? AND is_active = ?", userId, true).First(dest)
}

func FetchPassResetTokenByToken(hashedToken string, model any, dest any) *gorm.DB {
	return DB.Model(model).Where("token_hash = ?", hashedToken).First(dest)
}

func UpdateUserPass(database *gorm.DB, model any, userId uint, value map[string]any) *gorm.DB {
	return database.Model(model).Where("user_id = ?", userId).Select("password_hash", "pass_changed_at").Updates(value)
}

func DeletePassResetToken(database *gorm.DB, tokenId any, model any) *gorm.DB {
	return database.Model(model).Where("id = ?", tokenId).Delete(model)
}

func DeletePassResetTokenByUserId(database *gorm.DB, userId uint, model any) *gorm.DB {
	return database.Model(model).Where("user_id = ?", userId).Delete(model)
}

func FetchDeviceToken(token string, model any, dest any) *gorm.DB {
	return DB.Model(model).Where("token = ?", token).First(dest)
}

func DeactivateDeviceTokenBySessionId(sessionId string, model any) *gorm.DB {
	return DB.Model(model).Where("session_id = ?", sessionId).Updates(map[string]any{
		"is_active": false,
	})
}

func UpdateDeviceToken(database *gorm.DB, tokenId uint, value any, model any) *gorm.DB {
	return database.Model(model).Where("id = ?", tokenId).Updates(value)
}

func DeactivateDeviceToken(database *gorm.DB, token string, userId uint, model any) *gorm.DB {
	return database.Model(model).Where("token = ? AND user_id = ?", token, userId).Updates(map[string]any{
		"is_active": false,
	})
}

func FetchDeviceTokenByUserId(userId uint, model any, dest any) *gorm.DB {
	return DB.Model(model).Where("user_id = ? AND is_active = ?", userId, true).First(dest)
}
