package task

import (
	"errors"
	"fmt"
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

func GetWeeklyTaskHandler(c fiber.Ctx) error {

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
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{
				"message": "Task template not found",
				"error":   tx.Error.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	type OccKey struct {
		TaskID  uint
		DueDate time.Time
	}

	var wantedOccurrence []OccKey
	for _, template := range taskTemplates {

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

			for dueDate.Before(*now) {
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

			for dueDate.Before(*now) {
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

			for dueDate.Before(*now) {
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

	if tx := database.FetchOccurenceByUID(uid, &TaskOccurrence{}, &existingOccurence, *now, monthEnd); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
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

	for _, occurence := range missingOccurrence {
		if tx := database.Create(database.DB, &occurence, &TaskOccurrence{}); tx.Error != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Server error",
				"error":   tx.Error.Error(),
			})
		}
	}

	var uncompletedOcc []TaskOccurrence

	if tx := database.FetchUncompletedOccurrences(uid, &TaskOccurrence{}, &uncompletedOcc, monthEnd); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	overdue := make([]TaskOccurrence, 0)
	thisWeek := make([]TaskOccurrence, 0)
	thisMonth := make([]TaskOccurrence, 0)

	for _, occ := range uncompletedOcc {
		if occ.DueDate.Before(*now) {
			overdue = append(overdue, occ)
		} else if !occ.DueDate.After(weekEnd) {
			thisWeek = append(thisWeek, occ)
		} else {
			thisMonth = append(thisMonth, occ)
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
