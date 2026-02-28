package task

import (
	"fmt"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/modules/auth"
	"github.com/BarisNKorkmaz/taskManager/utils"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

func CreateTaskHandler(c fiber.Ctx) error {

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
		return c.Status(500).JSON(fiber.Map{
			"Message": "", // TODO error message
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
			if loc, err := time.LoadLocation(user.Timezone); err != nil {
				loc = time.UTC
				now := time.Now().In(loc)
				year, month, day := now.Date()
				dateOnly := time.Date(year, month, day, 0, 0, 0, 0, loc)
				data.StartDate = &dateOnly
			} else {
				now := time.Now().In(loc)
				year, month, day := now.Date()
				dateOnly := time.Date(year, month, day, 0, 0, 0, 0, loc)
				data.StartDate = &dateOnly
			}
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

	if tx := database.Create(task); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Task successfully created",
		"taskId":  task.ID,
	})

}
