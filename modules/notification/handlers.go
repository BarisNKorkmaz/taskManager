package notification

import (
	"errors"
	"fmt"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/utils"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func RegisterPushTokenHandler(c fiber.Ctx) error {
	data := new(RegisterPushTokenDTO)
	now := time.Now()
	uid := c.Locals("userId").(uint)

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
			messages = append(messages, fmt.Sprintf("Field: %s, Failed on: %s, on your value: %s", valErr.Field(), valErr.Tag(), valErr.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	deviceToken := new(DeviceToken)
	isTokenExist := true

	if tx := database.FetchDeviceToken(data.Token, &DeviceToken{}, deviceToken); tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			isTokenExist = false
			deviceToken.CreatedAt = now
		} else {
			return c.Status(500).JSON(fiber.Map{
				"message": "Server error",
				"error":   tx.Error.Error(),
			})
		}
	}

	deviceToken.AppVersion = data.AppVersion
	deviceToken.DeviceID = data.DeviceID
	deviceToken.IsActive = true
	deviceToken.Platform = data.Platform
	deviceToken.SessionID = c.Locals("sessionId").(string)
	deviceToken.Token = data.Token
	deviceToken.UserID = uid
	deviceToken.UpdatedAt = now
	deviceToken.LastSeenAt = now

	if isTokenExist {
		if tx := database.UpdateDeviceToken(database.DB, deviceToken.ID, deviceToken, &DeviceToken{}); tx.Error != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Server error",
				"error":   tx.Error.Error(),
			})
		} else {
			return c.Status(200).JSON(fiber.Map{
				"message": "Device Token successfully updated.",
			})
		}
	} else {
		if tx := database.Create(database.DB, deviceToken, &DeviceToken{}); tx.Error != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Server error",
				"error":   tx.Error.Error(),
			})
		}
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Device Token successfully created.",
	})
}

func DeletePushTokenHandler(c fiber.Ctx) error {
	data := new(DeletePushTokenDTO)
	uid := c.Locals("userId").(uint)

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
			messages = append(messages, fmt.Sprintf("Field: %s, Failed on: %s, On your value: %s", valErr.Field(), valErr.Tag(), valErr.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	if tx := database.DeactivateDeviceToken(database.DB, data.Token, uid, &DeviceToken{}); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "Device token successfully deactivated",
	})
}

func TestPushHandler(c fiber.Ctx) error {
	uid := c.Locals("userId").(uint)
	deviceToken := new(DeviceToken)

	if tx := database.FetchDeviceTokenByUserId(uid, &DeviceToken{}, deviceToken); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	id, err := SendPushToToken(deviceToken.Token, uid, "7Planner Test", "Push is working!!!")

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{
				"message": "anyactive FCM token not found for this user",
				"error":   err.Error(),
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   err.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"message":        "Notification sent successfully",
		"notificationId": id,
	})

}
