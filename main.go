package main

import (
	"os"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {

	middleware.Init()
	middleware.Log.Info("Logger initialized.")

	if err := godotenv.Load(); err != nil {
		middleware.Log.Error("Error loading .env file")
	}

	if err := database.Connect(); err != nil {
		middleware.Log.Error("Server starting operation failed:", "err", err.Error())
		os.Exit(1)
	} else {
		middleware.Log.Info("Database connected.")
	}

	app := fiber.New()
	app.Use(recoverer.New())
	app.Use(logger.New())

	app.Get("/", helloWorld)
	app.Get("/panictest", hPanic)

	port := os.Getenv("APP_PORT")
	if port == "" {
		middleware.Log.Error("APP_PORT env not set")
		os.Exit(1)
	}

	middleware.Log.Info("Starting server on :" + port)

	if err := app.Listen(":" + port); err != nil {
		middleware.Log.Error("Server failed to start", "err", err.Error())
		os.Exit(1)
	}

}

func hPanic(c fiber.Ctx) error {
	panic("panic")
}

func helloWorld(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "Hello World!",
	})
}
