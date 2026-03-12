package database

import (
	"errors"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() error {

	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")
	sslMode := os.Getenv("DB_SSLMODE")

	if host == "" || user == "" || pass == "" || dbname == "" || port == "" {
		return errors.New("required env not set")
	}
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", host, user, pass, dbname, port, sslMode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		return err
	}

	DB = db

	return err
}

func HealthHandler(c fiber.Ctx) error {

	sqlDB, err := DB.DB()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":   "error",
			"database": "not initialized",
		})
	}

	if err := sqlDB.Ping(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":   "error",
			"database": "disconnected",
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"status":   "ok",
		"database": "connected",
	})
}
