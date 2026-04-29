package task

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/middleware"
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

	data.RepeatType = strings.ToLower(data.RepeatType)
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
			"message": "Unauthorized or expired token",
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
	task.Category = CategoryType(data.Category)

	switch data.RepeatType {

	case "interval":
		if data.RepeatUnit == nil || *data.RepeatUnit == "" {
			return c.Status(400).JSON(fiber.Map{
				"message": "Bad request",
				"error":   "repeatUnit value must be one of = day, week or month when repeatType value is interval",
			})
		}

		if data.RepeatInterval == nil || *data.RepeatInterval <= 0 {
			return c.Status(400).JSON(fiber.Map{
				"message": "Bad request",
				"error":   "repeatInterval must be greater than 0 when repeatType is interval",
			})
		}

		if data.StartDate == nil {
			data.StartDate = getLocalDate(*user)
		} else {
			startDate, err := NormalizeToUserDate(*data.StartDate, user.Timezone)
			if err != nil {
				return c.Status(400).JSON(fiber.Map{
					"message": "Bad request",
					"error":   "invalid user timezone",
				})
			}
			data.StartDate = startDate
		}

		task.RepeatType = data.RepeatType
		task.RepeatUnit = data.RepeatUnit
		task.RepeatInterval = data.RepeatInterval
		task.StartDate = data.StartDate

	case "once":
		task.RepeatType = data.RepeatType
		if data.DueDate == nil {
			return c.Status(400).JSON(fiber.Map{
				"message": "if repeatType value is once, dueDate value can't be null",
			})
		}
		dueDate, err := NormalizeToUserDate(*data.DueDate, user.Timezone)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"message": "Bad request",
				"error":   "invalid user timezone",
			})
		}
		task.DueDate = dueDate

	case "weekly":
		task.RepeatType = data.RepeatType
		if data.WeekDays == nil || *data.WeekDays == "" {
			return c.Status(400).JSON(fiber.Map{
				"message": "if repeatType is weekly, weekDays value can't be null",
			})
		}

		days := strings.Split(*data.WeekDays, ",")
		seenDays := make(map[int]bool)

		for _, dayStr := range days {
			dayInt, err := strconv.Atoi(strings.TrimSpace(dayStr))

			if err != nil || dayInt < 0 || dayInt > 6 {
				return c.Status(400).JSON(fiber.Map{
					"message": "Bad request",
					"error":   fmt.Sprintf("invalid weekDay value: %s. Must be numbers between 0-6 separated by commas", dayStr),
				})
			}

			if seenDays[dayInt] {
				return c.Status(400).JSON(fiber.Map{
					"message": "Bad request",
					"error":   fmt.Sprintf("duplicate day detected: %d. Each day must be unique", dayInt),
				})
			}
			seenDays[dayInt] = true
		}

		task.StartDate = getLocalDate(*user)
		task.WeekDays = data.WeekDays

	default:
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   "invalid repeatType",
		})
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

	if task.RepeatType == "once" {

		occ := TaskOccurrence{
			TaskID:  task.ID,
			UserID:  userID,
			DueDate: *task.DueDate,
			Status:  "pending",
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
				Category:    taskTemplatesMap[occ.TaskID].Category,
				DueDate:     occ.DueDate,
				Status:      occ.Status,
			})
		} else if !occ.DueDate.After(weekEnd) {
			thisWeek = append(thisWeek, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				Category:    taskTemplatesMap[occ.TaskID].Category,
				DueDate:     occ.DueDate,
				Status:      occ.Status,
			})
		} else {
			thisMonth = append(thisMonth, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				Category:    taskTemplatesMap[occ.TaskID].Category,
				DueDate:     occ.DueDate,
				Status:      occ.Status,
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
		if data.NewDueDate == nil {
			return c.Status(400).JSON(fiber.Map{
				"message": "Bad request",
				"error":   "newDueDate can't be null",
			})
		} else {
			if dueDate, err := NormalizeToUserDate(*data.NewDueDate, user.Timezone); err != nil {
				return c.Status(400).JSON(fiber.Map{
					"message": "Bad request",
					"error":   "invalid user timezone",
				})
			} else {
				data.NewDueDate = dueDate
				if data.NewDueDate.Before(*now) {
					return c.Status(400).JSON(fiber.Map{
						"message": "Bad request",
						"error":   "newDueDate cannot be in the past",
					})
				}
			}
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

		if occ.Status == "completed" {
			return c.Status(409).JSON(fiber.Map{
				"message": "Conflict",
				"error":   "occurrence is already completed",
			})
		}

		occ.Status = "completed"
		occ.CompletedAt = now

		resMessage = "task completed"

	case "undo":
		if occ.Status == "pending" {
			return c.Status(409).JSON(fiber.Map{
				"message": "Conflict",
				"error":   "occurrence is already pending",
			})
		}
		occ.Status = "pending"
		occ.CompletedAt = nil

		resMessage = "task completion undone"

	case "skip":
		if occ.Status == "skipped" {
			return c.Status(409).JSON(fiber.Map{
				"message": "Conflict",
				"error":   "occurrence is already skipped",
			})
		}
		occ.Status = "skipped"
		occ.CompletedAt = nil
		resMessage = "task skipped"

	case "reschedule":
		occ.DueDate = *data.NewDueDate
		occ.Status = "pending"
		occ.CompletedAt = nil
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
		} else if tx.RowsAffected == 0 {
			return c.Status(404).JSON(fiber.Map{
				"message": "Not found",
				"error":   "occurrence could not be updated",
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
		"status":       occ.Status,
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

	if data.Description == nil && data.DueDate == nil && data.RepeatInterval == nil && data.RepeatUnit == nil && data.StartDate == nil && data.Title == nil && data.Category == nil {
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

	switch template.RepeatType {
	case "interval":
		data.DueDate = nil
	case "once":
		data.RepeatInterval = nil
		data.RepeatUnit = nil
		data.StartDate = nil
	case "weekly":
		data.RepeatInterval = nil
		data.RepeatUnit = nil
		data.StartDate = nil
		data.DueDate = nil
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

		if template.RepeatType == "once" {

			if tx := database.FetchTaskTemplateById(&TaskTemplate{}, taskId, uid, template); tx.Error != nil {
				atomicDB.Rollback()
				return c.Status(500).JSON(fiber.Map{
					"message": "Server error",
					"error":   tx.Error.Error(),
				})
			}

			newDueDate := template.DueDate // Default olarak eskisi kalsın
			if data.DueDate != nil {
				newDueDate = data.DueDate
			}

			occ := TaskOccurrence{
				TaskID:  template.ID,
				UserID:  uid,
				DueDate: *newDueDate,
				Status:  "pending",
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

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)

		if fieldVal.Kind() == reflect.Ptr {
			if !fieldVal.IsNil() {
				key := fieldType.Tag.Get("json")
				if key == "" || key == "-" {
					key = fieldType.Name
				} else {
					key = strings.Split(key, ",")[0]
				}

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

	fmt.Printf("DEBUG: Incoming IsActive: %v, DB IsActive: %v\n", *data.IsActive, template.IsActive)

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
		fmt.Printf("DEBUG: DueDate: %v, Now: %v, Before: %v\n", template.DueDate, *now, template.DueDate.Before(*now))
		if template.RepeatType == "once" && !template.DueDate.Before(*now) {
			occ := &TaskOccurrence{
				TaskID:  template.ID,
				UserID:  uid,
				DueDate: *template.DueDate,
				Status:  "pending",
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

func generateOcc(taskTemplates []TaskTemplate, uid uint, now time.Time, LimitTime time.Time) (map[uint]TaskTemplate, error) {
	type OccKey struct {
		TaskID  uint
		DueDate time.Time
	}

	taskTemplatesMap := make(map[uint]TaskTemplate)

	var wantedOccurrence []OccKey
	for _, template := range taskTemplates {

		taskTemplatesMap[template.ID] = template

		if template.RepeatType == "once" || !template.IsActive {
			continue
		}

		switch strings.ToLower(template.RepeatType) {

		case "weekly":

			selectedDays := make(map[time.Weekday]bool)

			parsedWeekDays := strings.ReplaceAll(*template.WeekDays, " ", "")

			for _, dayStr := range strings.Split(parsedWeekDays, ",") {
				dayInt, _ := strconv.Atoi(dayStr)
				selectedDays[time.Weekday(dayInt)] = true
			}

			currentWeekStart, _ := FindWeekStartAndEndDay(now)
			for i := currentWeekStart; i.Before(LimitTime); i = i.AddDate(0, 0, 1) {
				dueDate := i

				if dueDate.Before(now) {
					continue
				}

				if selectedDays[dueDate.Weekday()] {
					wantedOccurrence = append(wantedOccurrence, OccKey{
						TaskID:  template.ID,
						DueDate: dueDate,
					})
				}
			}

		case "interval":
			if template.RepeatUnit == nil || template.StartDate == nil || template.RepeatInterval == nil || *template.RepeatInterval <= 0 {
				break
			}
			switch strings.ToLower(*template.RepeatUnit) {
			case "day":
				dueDate := *template.StartDate

				for dueDate.Before(now) {
					dueDate = dueDate.AddDate(0, 0, *template.RepeatInterval)
				}

				for !dueDate.After(LimitTime) {
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

				for !dueDate.After(LimitTime) {
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

				for !dueDate.After(LimitTime) {
					wantedOccurrence = append(wantedOccurrence, OccKey{
						TaskID:  template.ID,
						DueDate: dueDate,
					})
					dueDate = dueDate.AddDate(0, *template.RepeatInterval, 0)
				}

			}

		default:
			middleware.Log.Error("Unexpected repeatType", "templateId", template.ID, "userId", uid, "repeatType", template.RepeatType)
		}

	}

	var existingOccurence []OccKey
	if tx := database.FetchOccurenceByUID(uid, &TaskOccurrence{}, &existingOccurence, now, LimitTime); tx.Error != nil {
		fmt.Println("FetchOccurenceByUID error")
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
			TaskID:  wanted.TaskID,
			UserID:  uid,
			DueDate: wanted.DueDate,
			Status:  "pending",
		})
	}
	if len(missingOccurrence) > 0 {
		if tx := database.CreateOccurrencesBatch(database.DB, missingOccurrence, &TaskOccurrence{}, 200); tx.Error != nil && !strings.Contains(tx.Error.Error(), "SQLSTATE 23505") {
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

	if tx := database.FetchTasksByUID(uid, &TaskTemplate{}, &taskTemplates); tx.Error != nil {
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

		dueNorm, _ := NormalizeToUserDate(occ.DueDate, user.Timezone)

		if dueNorm.Before(*today) {
			overdue = append(overdue, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				Category:    taskTemplatesMap[occ.TaskID].Category,
				DueDate:     occ.DueDate,
				Status:      occ.Status,
			})
		} else if !dueNorm.After(*today) || occ.DueDate.Equal(*today) {
			todayOccs = append(todayOccs, DashboardOccurrenceDTO{
				ID:          occ.ID,
				TaskID:      occ.TaskID,
				Title:       taskTemplatesMap[occ.TaskID].Title,
				Description: taskTemplatesMap[occ.TaskID].Description,
				Category:    taskTemplatesMap[occ.TaskID].Category,
				DueDate:     occ.DueDate,
				Status:      occ.Status,
			})
		} /* else {
			fmt.Println("----------------------------DEBUG TIME-------------------------------------------")
			fmt.Println(occ.TaskID)
			fmt.Println(occ.DueDate)
			fmt.Println(dueNorm)
			fmt.Println(*today)
			fmt.Println(" ")
		} */
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

func NormalizeToUserDate(t time.Time, userTimezone string) (*time.Time, error) {
	loc, err := time.LoadLocation(userTimezone)
	if err != nil {
		return nil, err
	}

	local := t.In(loc)

	normalized := time.Date(
		local.Year(),
		local.Month(),
		local.Day(),
		0, 0, 0, 0,
		loc,
	)

	return &normalized, nil
}

func WeeklyReportHandler(c fiber.Ctx) error {

	uid := c.Locals("userId").(uint)
	user := new(auth.User)

	if tx := database.FetchUserByUID(uid, user); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	now := getLocalDate(*user)
	weekStartDay, weekEndDay := FindWeekStartAndEndDay(*now)

	var occurrences []TaskOccurrence
	var overdueOccurrences []TaskOccurrence
	var completedOnTime []map[string]any
	var completedLate []map[string]any
	var skipped []map[string]any
	var overdue []map[string]any

	if tx := database.FetchWeeklyOccurrences(uid, &TaskOccurrence{}, weekStartDay, weekEndDay, &occurrences); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"Message": "Server error",
			"error":   tx.Error,
		})
	}

	if tx := database.FetchOverdueOccurrences(uid, &TaskOccurrence{}, weekEndDay, &overdueOccurrences); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"Message": "Server error",
			"error":   tx.Error,
		})
	}

	templatesTitle := make(map[uint]string)
	score := 0
	for _, occ := range occurrences {
		if templatesTitle[occ.TaskID] == "" {
			template := new(TaskTemplate)
			if tx := database.FetchTaskTemplateById(&TaskTemplate{}, occ.TaskID, uid, template); tx.Error != nil {
				return c.Status(500).JSON(fiber.Map{
					"message": "Server error",
					"error":   tx.Error.Error(),
				})
			}
			templatesTitle[occ.TaskID] = template.Title
		}

		switch strings.ToLower(occ.Status) {
		case "completed":

			if occ.CompletedAt.After(occ.DueDate) {
				score += 4
				completedLate = append(completedLate, map[string]any{
					"title":    templatesTitle[occ.TaskID],
					"due":      occ.DueDate.Weekday().String(),
					"actual":   occ.CompletedAt.Weekday().String(),
					"daysDiff": fmt.Sprintf("%v days", math.Round(occ.CompletedAt.Sub(occ.DueDate).Hours()/24)),
				})
			} else {
				score += 10
				completedOnTime = append(completedOnTime, map[string]any{
					"title": templatesTitle[occ.TaskID],
					"date":  occ.CompletedAt.Format("02-01-2006"),
				})
			}

		case "skipped":
			skipped = append(skipped, map[string]any{
				"title": templatesTitle[occ.TaskID],
			})

		case "pending":

		}

	}

	for _, occ := range overdueOccurrences {
		if templatesTitle[occ.TaskID] == "" {
			template := new(TaskTemplate)
			if tx := database.FetchTaskTemplateById(&TaskTemplate{}, occ.TaskID, uid, template); tx.Error != nil {
				return c.Status(500).JSON(fiber.Map{
					"message": "Server error",
					"error":   tx.Error.Error(),
				})
			}
			templatesTitle[occ.TaskID] = template.Title
		}

		score -= 5
		overdue = append(overdue, map[string]any{
			"title":      templatesTitle[occ.TaskID],
			"dueDate":    occ.DueDate.Format("02-01-2006"),
			"waitingFor": fmt.Sprintf("%v days", math.Round(now.Sub(occ.DueDate).Hours()/24)),
		})
	}

	totalPossible := float64(len(occurrences) * 10)
	finalScore := 0
	if totalPossible > 0 {
		finalScore = max(int((float64(score)/totalPossible)*100), 0)
	}

	return c.Status(200).JSON(fiber.Map{
		"period": fmt.Sprintf("%s - %s", weekStartDay.Format("02-01-2006"), weekEndDay.Format("02-01-2006")),
		"stats": map[string]int{
			"total_count":     len(occurrences),
			"completedOnTime": len(completedOnTime),
			"completedLate":   len(completedLate),
			"overduePending":  len(overdue),
			"skipped":         len(skipped),
			"score":           finalScore,
		},
		"details": map[string][]map[string]any{
			"completed_on_time": completedOnTime,
			"completedLate":     completedLate,
			"overduePending":    overdue,
			"skipped":           skipped,
		},
	})

}

func FindWeekStartAndEndDay(now time.Time) (weekStartDay time.Time, weekEndDay time.Time) {
	weekDay := int(now.Weekday())

	if weekDay == 0 {
		weekDay = 7
	}

	weekStartDay = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(weekDay - 1))
	weekEndDay = weekStartDay.AddDate(0, 0, 6)

	return
}

func GenerateDailyOccs(userId uint) (int, error) {
	var taskTemplates []TaskTemplate

	if tx := database.FetchTaskTemplates(&TaskTemplate{}, userId, &taskTemplates); tx.Error != nil {
		return 0, tx.Error
	}

	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := now.AddDate(0, 0, 1)

	templateMap, err := generateOcc(taskTemplates, userId, now, tomorrow)

	if err != nil {
		return 0, err
	}

	count := len(templateMap)

	return count, nil

}
