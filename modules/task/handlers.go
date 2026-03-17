package task

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/modules/auth"
	"github.com/BarisNKorkmaz/taskManager/utils"
	"github.com/duke-git/lancet/v2/datetime"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func CreateTaskTemplateHandler(c fiber.Ctx) error {

	data := new(TaskDTO)

	if err := c.Bind().Body(data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	if errs := utils.Validate.Struct(data); errs != nil {
		var messages []string
		valErrs := errs.(validator.ValidationErrors)

		for _, valErr := range valErrs {
			messages = append(messages, fmt.Sprintf("Field: %s, failed on: %s, on your value: %s", valErr.Field(), valErr.Tag(), valErr.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	userID, ok := c.Locals("userId").(uint)

	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"Message": "Unauthorized or expired token",
		})
	}
	user := &auth.User{}

	if tx := database.FetchUserByUID(userID, user); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	task := new(TaskTemplate)
	task.UserID = userID
	task.Title = data.Title
	task.Description = data.Description

	if data.IsRepeatEnabled {

		if data.RepeatUnit == nil {
			return c.Status(400).JSON(fiber.Map{
				"message": "Bad request",
				"error":   "repeatUnit value must be one of = day, week or month when isRepeatEnabled is true",
			})
		}

		if data.RepeatInterval == nil {
			return c.Status(400).JSON(fiber.Map{
				"message": "Bad request",
				"error":   "repeatInterval can't be null when isRepeatEnabled is true",
			})
		}

		if data.StartDate == nil {
			data.StartDate = getLocalDate(*user)
		}

		task.IsRepeatEnabled = true
		task.RepeatUnit = data.RepeatUnit
		task.RepeatInterval = data.RepeatInterval
		task.StartDate = data.StartDate
	} else {
		task.IsRepeatEnabled = false
		if data.DueDate == nil {
			return c.Status(400).JSON(fiber.Map{
				"message": "if isRepeatEnabled is false, dueDate value can't be null",
			})
		}
		task.DueDate = data.DueDate
	}

	atomicDB := database.DB.Begin()
	if atomicDB.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   atomicDB.Error.Error(),
		})
	}

	if tx := database.Create(atomicDB, task, &TaskTemplate{}); tx.Error != nil {
		atomicDB.Rollback()
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if !task.IsRepeatEnabled {
		occ := TaskOccurrence{
			TaskID:      task.ID,
			UserID:      userID,
			DueDate:     *task.DueDate,
			IsCompleted: false,
		}

		if tx := database.Create(atomicDB, &occ, &TaskOccurrence{}); tx.Error != nil {
			atomicDB.Rollback()
			return c.Status(500).JSON(fiber.Map{
				"message": "Server error",
				"error":   tx.Error.Error(),
			})
		}
	}

	if res := atomicDB.Commit(); res.Error != nil {
		atomicDB.Rollback()
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   res.Error.Error(),
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Task successfully created",
		"taskId":  task.ID,
	})

}

func DashboardHandler(c fiber.Ctx) error {

	user := new(auth.User)
	uid := c.Locals("userId").(uint)

	if tx := database.FetchUserByUID(uid, user); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	now := getLocalDate(*user)
	weekEnd := datetime.EndOfWeek(*now, time.Sunday)
	monthEnd := datetime.EndOfMonth(*now)

	fmt.Printf("now: %s, weekend: %s, monthend: %s", now.String(), weekEnd.String(), monthEnd.String())

	var taskTemplates []TaskTemplate
	if tx := database.FetchTasksByUID(uid, &TaskTemplate{}, &taskTemplates); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	taskTemplatesMap, generateErr := generateOcc(taskTemplates, uid, *now, monthEnd)

	if generateErr != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   generateErr.Error(),
		})
	}

	var uncompletedOcc []TaskOccurrence

	if tx := database.FetchUncompletedOccurrences(uid, &TaskOccurrence{}, &uncompletedOcc, monthEnd); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	overdue := make([]DashboardOccurrenceDTO, 0)
	thisWeek := make([]DashboardOccurrenceDTO, 0)
	thisMonth := make([]DashboardOccurrenceDTO, 0)

	for _, occ := range uncompletedOcc {
		if occ.DueDate.Before(*now) {
			overdue = append(overdue, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				DueDate:     occ.DueDate,
				IsCompleted: occ.IsCompleted,
			})
		} else if !occ.DueDate.After(weekEnd) {
			thisWeek = append(thisWeek, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				DueDate:     occ.DueDate,
				IsCompleted: occ.IsCompleted,
			})
		} else {
			thisMonth = append(thisMonth, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				DueDate:     occ.DueDate,
				IsCompleted: occ.IsCompleted,
			})
		}
	}

	return c.Status(200).JSON(fiber.Map{
		"today":    (*now).Format("2006-01-02"),
		"weekEnd":  weekEnd.Format("2006-01-02"),
		"monthEnd": monthEnd.Format("2006-01-02"),

		"overdue": overdue,
		"weekly":  thisWeek,
		"monthly": thisMonth,

		"counts": fiber.Map{
			"overdue": len(overdue),
			"month":   len(thisMonth),
			"week":    len(thisWeek),
		},
	})

}

func getLocalDate(user auth.User) *time.Time {

	if loc, err := time.LoadLocation(user.Timezone); err != nil {
		loc = time.UTC
		now := time.Now().In(loc)
		year, month, day := now.Date()
		dateOnly := time.Date(year, month, day, 0, 0, 0, 0, loc)
		return &dateOnly
	} else {
		now := time.Now().In(loc)
		year, month, day := now.Date()
		dateOnly := time.Date(year, month, day, 0, 0, 0, 0, loc)
		return &dateOnly
	}

}

func UpdateOccStatusHandler(c fiber.Ctx) error {

	occIdStr := c.Params("id")
	occId64, uintParseErr := strconv.ParseUint(occIdStr, 10, 64)
	if uintParseErr != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   uintParseErr.Error(),
		})
	}
	uid := c.Locals("userId").(uint)
	data := new(TaskActionDTO)

	if err := c.Bind().Body(data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	if err := utils.Validate.Struct(data); err != nil {
		var messages []string
		ValErrs := err.(validator.ValidationErrors)

		for _, valErr := range ValErrs {
			messages = append(messages, fmt.Sprintf("Field: %s, failed on: %s, on your value: %v", valErr.Field(), valErr.Tag(), valErr.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	user := new(auth.User)
	if tx := database.FetchUserByUID(uid, user); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}
	now := getLocalDate(*user)
	action := strings.ToLower(data.Action)
	if action == "reschedule" {
		if data.NewDueDate == nil || !data.NewDueDate.After(*now) {
			return c.Status(400).JSON(fiber.Map{
				"message": "Bad request",
				"error":   "newDueDate must be a future date for reschedule",
			})
		}
	}
	occ := new(TaskOccurrence)
	if tx := database.FetchOccurenceByOccId(&TaskOccurrence{}, occId64, uid, occ); tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{
				"message": "Not found",
				"error":   tx.Error.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	var resMessage string

	switch action {
	case "complete":

		occ.IsCompleted = true
		occ.CompletedAt = now

		resMessage = "task completed"

	case "undo":
		if occ.IsCompleted {
			return c.Status(409).JSON(fiber.Map{
				"message": "Conflict",
				"error":   "occurrence is already pending",
			})
		}
		occ.IsCompleted = false
		occ.CompletedAt = nil

		resMessage = "task completion undone"

	case "skip":
		occ.IsCompleted = true
		occ.CompletedAt = now

		resMessage = "task skipped"

	case "reschedule":
		occ.DueDate = *data.NewDueDate

		resMessage = "task rescheduled"

	default:

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   "action is must be one of = complete undo skip reschedule",
		})

	}

	if tx := database.UpdateOccStatus(&TaskOccurrence{}, occId64, occ); tx.Error != nil || tx.RowsAffected == 0 {
		if errors.Is(tx.Error, gorm.ErrDuplicatedKey) {
			return c.Status(409).JSON(fiber.Map{
				"message": "Conflict",
				"error":   "another occurrence already exists for this task on the selected date",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"message":      resMessage,
		"action":       action,
		"occurrenceId": occ.ID,
		"status":       occ.IsCompleted,
		"dueDate":      occ.DueDate,
		"completedAt":  occ.CompletedAt,
	})

}

func UpdateTaskTemplateHandler(c fiber.Ctx) error {
	data := new(UpdateTemplateDTO)
	uid := c.Locals("userId").(uint)
	taskIdStr := c.Params("id")
	taskId, parseErr := strconv.ParseUint(taskIdStr, 10, 64)
	if parseErr != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   parseErr.Error(),
		})
	}

	if err := c.Bind().Body(data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	if err := utils.Validate.Struct(data); err != nil {
		var messages []string
		valErrs := err.(validator.ValidationErrors)

		for _, valErr := range valErrs {
			messages = append(messages, fmt.Sprintf("Field: %s, failed on: %s, on your value: %v", valErr.Field(), valErr.Tag(), valErr.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	if data.Description == nil && data.DueDate == nil && data.RepeatInterval == nil && data.RepeatUnit == nil && data.StartDate == nil && data.Title == nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   "Invalid request payload. Please check your data format.",
		})
	}

	template := new(TaskTemplate)
	if tx := database.FetchTaskTemplateById(&TaskTemplate{}, taskId, uid, template); tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{
				"message": "Not found",
				"error":   tx.Error.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if template.IsRepeatEnabled {
		data.DueDate = nil
	} else {
		data.RepeatInterval = nil
		data.RepeatUnit = nil
		data.StartDate = nil
	}

	user := new(auth.User)

	if tx := database.FetchUserByUID(uid, user); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	now := getLocalDate(*user)

	updates := StructToUpdateMap(data)

	shouldDeleteFutureOccs := data.DueDate != nil ||
		data.RepeatUnit != nil ||
		data.RepeatInterval != nil ||
		data.StartDate != nil

	atomicDB := database.DB.Begin()
	if tx := database.UpdateTaskTemplate(atomicDB, &TaskTemplate{}, taskId, uid, updates); tx.Error != nil {
		atomicDB.Rollback()
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}
	if shouldDeleteFutureOccs {
		if tx := database.DeleteChangedOccs(atomicDB, &TaskOccurrence{}, taskId, *now, uid); tx.Error != nil {
			atomicDB.Rollback()
			return c.Status(500).JSON(fiber.Map{
				"message": "Server error",
				"error":   tx.Error.Error(),
			})
		}

		if !template.IsRepeatEnabled {

			if tx := database.FetchTaskTemplateById(&TaskTemplate{}, taskId, uid, template); tx.Error != nil {
				atomicDB.Rollback()
				return c.Status(500).JSON(fiber.Map{
					"message": "Server error",
					"error":   tx.Error.Error(),
				})
			}

			occ := TaskOccurrence{
				TaskID:      template.ID,
				UserID:      uid,
				DueDate:     *template.DueDate,
				IsCompleted: false,
			}

			if tx := database.Create(atomicDB, &occ, &TaskOccurrence{}); tx.Error != nil {
				atomicDB.Rollback()
				return c.Status(500).JSON(fiber.Map{
					"message": "Server error",
					"error":   tx.Error.Error(),
				})
			}

		}
	}

	if res := atomicDB.Commit(); res.Error != nil {
		atomicDB.Rollback()
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   res.Error.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "Task template successfully updated",
		"taskId":  taskId,
	})

}

func StructToUpdateMap(data interface{}) map[string]any {
	update := make(map[string]any)

	v := reflect.ValueOf(data)
	t := reflect.TypeOf(data)

	// Eğer data bir pointer ise asıl objeye git (Dereference)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)  // Alanın değeri
		fieldType := t.Field(i) // Alanın meta verisi (Tag vb.)

		// Sadece pointer olan ve nil olmayan alanları kontrol et
		if fieldVal.Kind() == reflect.Ptr {
			if !fieldVal.IsNil() {
				// JSON tag'ini anahtar olarak al, yoksa field ismini kullan
				key := fieldType.Tag.Get("json")
				if key == "" || key == "-" {
					key = fieldType.Name
				} else {
					// "title,omitempty" gibi durumlarda virgül sonrasını at
					key = strings.Split(key, ",")[0]
				}

				// Pointer'ın gösterdiği asıl değeri map'e ekle
				update[key] = fieldVal.Elem().Interface()
			}
		}
	}
	return update
}

func SetTaskTemplateStatusHandler(c fiber.Ctx) error {
	data := new(SetTemplateStatusDTO)
	TaskIdStr := c.Params("id")
	taskId, parseErr := strconv.ParseUint(TaskIdStr, 10, 64)
	if parseErr != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   parseErr.Error(),
		})
	}

	uid := c.Locals("userId").(uint)

	if err := c.Bind().Body(data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	if valErr := utils.Validate.Struct(data); valErr != nil {
		var messages []string
		valErrs := valErr.(validator.ValidationErrors)
		for _, err := range valErrs {
			messages = append(messages, fmt.Sprintf("Field: %s, failed on: %s, on your value: %v", err.Field(), err.Tag(), err.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	template := new(TaskTemplate)

	if tx := database.FetchTaskTemplateById(&TaskTemplate{}, taskId, uid, template); tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{
				"message": "Not found",
				"error":   tx.Error.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if template.IsActive == *data.IsActive {
		return c.Status(200).JSON(fiber.Map{
			"message":  "Task status already set",
			"taskId":   taskId,
			"isActive": template.IsActive,
		})
	}

	update := map[string]any{
		"is_active": data.IsActive,
	}

	user := new(auth.User)

	if tx := database.FetchUserByUID(uid, user); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	now := getLocalDate(*user)

	atomicDb := database.DB.Begin()
	if tx := database.UpdateTaskTemplate(atomicDb, &TaskTemplate{}, taskId, uid, update); tx.Error != nil {
		atomicDb.Rollback()
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if !(*data.IsActive) {
		if tx := database.DeleteChangedOccs(atomicDb, &TaskOccurrence{}, taskId, *now, uid); tx.Error != nil {
			atomicDb.Rollback()
			return c.Status(500).JSON(fiber.Map{
				"message": "Server error",
				"error":   tx.Error.Error(),
			})
		}
	} else {
		if !template.IsRepeatEnabled && !template.DueDate.Before(*now) {
			occ := &TaskOccurrence{
				TaskID:      template.ID,
				UserID:      uid,
				DueDate:     *template.DueDate,
				IsCompleted: false,
			}

			if tx := database.Create(atomicDb, occ, &TaskOccurrence{}); tx.Error != nil {
				atomicDb.Rollback()
				return c.Status(500).JSON(fiber.Map{
					"message": "Server error",
					"error":   tx.Error.Error(),
				})
			}
		}
	}

	if res := atomicDb.Commit(); res.Error != nil {
		atomicDb.Rollback()
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   res.Error.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"message":  "Task status successfully changed",
		"taskId":   taskId,
		"isActive": data.IsActive,
	})

}

func GetTaskTemplatesHandler(c fiber.Ctx) error {
	uid := c.Locals("userId").(uint)

	var templates []TaskTemplate
	var activeTemplates []TaskTemplate
	var inactiveTemplates []TaskTemplate

	if tx := database.FetchTaskTemplates(&TaskTemplate{}, uid, &templates); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	for _, template := range templates {
		if template.IsActive {
			activeTemplates = append(activeTemplates, template)
		} else {
			inactiveTemplates = append(inactiveTemplates, template)
		}
	}

	return c.Status(200).JSON(fiber.Map{
		"message":               "Task templates fetched successfully",
		"activeTemplates":       activeTemplates,
		"inactiveTemplates":     inactiveTemplates,
		"activeTemplateCount":   len(activeTemplates),
		"inactiveTemplateCount": len(inactiveTemplates),
		"templateCount":         len(templates),
	})

}

// GET /taskTemplates/:id
func GetTemplateDetailHandler(c fiber.Ctx) error {
	uid := c.Locals("userId").(uint)
	templateIdStr := c.Params("id")
	templateId, ok := strconv.ParseUint(templateIdStr, 10, 64)
	if ok != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   ok.Error(),
		})
	}

	template := new(TaskTemplate)

	if tx := database.FetchTaskTemplateById(&TaskTemplate{}, templateId, uid, template); tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{
				"message": "Record not fount",
				"error":   tx.Error.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "Task template fetched successfully",
		"data":    template,
	})

}

func generateOcc(taskTemplates []TaskTemplate, uid uint, now time.Time, monthEnd time.Time) (map[uint]TaskTemplate, error) {
	type OccKey struct {
		TaskID  uint
		DueDate time.Time
	}

	taskTemplatesMap := make(map[uint]TaskTemplate)

	var wantedOccurrence []OccKey
	for _, template := range taskTemplates {

		taskTemplatesMap[template.ID] = template

		if !template.IsRepeatEnabled || !template.IsActive {
			continue
		}
		if template.RepeatUnit == nil || template.StartDate == nil || template.RepeatInterval == nil {
			continue
		}
		if *template.RepeatInterval <= 0 {
			continue
		}

		switch strings.ToLower(*template.RepeatUnit) {

		case "day":

			dueDate := *template.StartDate

			for dueDate.Before(now) {
				dueDate = dueDate.AddDate(0, 0, *template.RepeatInterval)
			}

			for !dueDate.After(monthEnd) {
				wantedOccurrence = append(wantedOccurrence, OccKey{
					TaskID:  template.ID,
					DueDate: dueDate,
				})
				dueDate = dueDate.AddDate(0, 0, *template.RepeatInterval)
			}

		case "week":

			dueDate := *template.StartDate

			for dueDate.Before(now) {
				dueDate = dueDate.AddDate(0, 0, 7*(*template.RepeatInterval))
			}

			for !dueDate.After(monthEnd) {
				wantedOccurrence = append(wantedOccurrence, OccKey{
					TaskID:  template.ID,
					DueDate: dueDate,
				})
				dueDate = dueDate.AddDate(0, 0, 7*(*template.RepeatInterval))
			}

		case "month":
			dueDate := *template.StartDate

			for dueDate.Before(now) {
				dueDate = dueDate.AddDate(0, *template.RepeatInterval, 0)
			}

			for !dueDate.After(monthEnd) {
				wantedOccurrence = append(wantedOccurrence, OccKey{
					TaskID:  template.ID,
					DueDate: dueDate,
				})
				dueDate = dueDate.AddDate(0, *template.RepeatInterval, 0)
			}

		}

	}

	var existingOccurence []OccKey

	if tx := database.FetchOccurenceByUID(uid, &TaskOccurrence{}, &existingOccurence, now, monthEnd); tx.Error != nil {
		return nil, tx.Error
	}

	existingSet := make(map[string]struct{}, len(existingOccurence))

	for _, e := range existingOccurence {
		key := fmt.Sprintf("%d|%s", e.TaskID, e.DueDate.Format("2006-01-02"))
		existingSet[key] = struct{}{}
	}

	var missingOccurrence []TaskOccurrence

	for _, wanted := range wantedOccurrence {
		key := fmt.Sprintf("%d|%s", wanted.TaskID, wanted.DueDate.Format("2006-01-02"))

		if _, ok := existingSet[key]; ok {
			continue
		}

		missingOccurrence = append(missingOccurrence, TaskOccurrence{
			TaskID:      wanted.TaskID,
			UserID:      uid,
			DueDate:     wanted.DueDate,
			IsCompleted: false,
		})
	}
	if len(missingOccurrence) > 0 {
		if tx := database.CreateOccurrencesBatch(database.DB, missingOccurrence, &TaskOccurrence{}, 200); tx.Error != nil {
			return nil, tx.Error
		}
	}

	return taskTemplatesMap, nil
}

func GetTodayOccs(c fiber.Ctx) error {
	uid := c.Locals("userId").(uint)

	user := new(auth.User)

	if tx := database.FetchUserByUID(uid, user); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	today := getLocalDate(*user)
	monthEnd := datetime.EndOfMonth(*today)

	var taskTemplates []TaskTemplate
	var taskTemplatesMap map[uint]TaskTemplate

	if tx := database.FetchTaskTemplates(&TaskTemplate{}, uid, &taskTemplates); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	taskTemplatesMap, generateErr := generateOcc(taskTemplates, uid, *today, monthEnd)

	if generateErr != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   generateErr.Error(),
		})
	}

	var uncompletedOcc []TaskOccurrence

	if tx := database.FetchUncompletedOccurrences(uid, &TaskOccurrence{}, &uncompletedOcc, *today); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	overdue := make([]DashboardOccurrenceDTO, 0)
	todayOccs := make([]DashboardOccurrenceDTO, 0)

	for _, occ := range uncompletedOcc {

		if occ.DueDate.Before(*today) {
			overdue = append(overdue, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				DueDate:     occ.DueDate,
				IsCompleted: occ.IsCompleted,
			})
		} else if !occ.DueDate.After(*today) {
			todayOccs = append(todayOccs, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				DueDate:     occ.DueDate,
				IsCompleted: occ.IsCompleted,
			})
		}
	}

	return c.Status(200).JSON(fiber.Map{
		"today":       todayOccs,
		"overdue":     overdue,
		"currentDate": today,

		"counts": fiber.Map{
			"overdue": len(overdue),
			"today":   len(todayOccs),
		},
	})

}
