package main

import (
	"os"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/middleware"
	"github.com/BarisNKorkmaz/taskManager/modules/auth"
	"github.com/BarisNKorkmaz/taskManager/modules/task"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {

	middleware.Init()
	middleware.Log.Info("Logger initialized.")

	if err := godotenv.Load(); err != nil {
		middleware.Log.Error("Error loading .env file", "err", err)
	}

	if err := database.Connect(); err != nil {
		middleware.Log.Error("Server starting operation failed:", "err", err.Error())
		os.Exit(1)
	} else {
		middleware.Log.Info("Database connected.")
	}

	if err := database.DB.AutoMigrate(&auth.User{}, &task.TaskTemplate{}, &task.TaskOccurrence{}); err != nil {
		middleware.Log.Error("Migration error:", "err", err.Error())
	} else {
		middleware.Log.Info("Database migrated")
	}

	app := fiber.New()
	app.Use(recoverer.New())
	app.Use(logger.New())
	app.Get("/health", database.HealthHandler)

	app.Post("/register", auth.RegisterHandler)
	app.Post("/login", auth.LoginHandler)

	protected := app.Group("/u", auth.JWTMiddleware())

	protected.Get("/auth/me", auth.MeHandler)

	protected.Post("/tasks/templates", task.CreateTaskTemplateHandler)
	protected.Get("/tasks/templates", task.GetTaskTemplatesHandler)
	protected.Get("/tasks/templates/:id", task.GetTemplateDetailHandler)
	protected.Patch("/tasks/templates/:id", task.UpdateTaskTemplateHandler)
	protected.Patch("/tasks/templates/:id/status", task.SetTaskTemplateStatusHandler)

	protected.Get("/dashboard", task.DashboardHandler)
	protected.Get("/tasks/today", task.GetTodayOccs)

	protected.Patch("/tasks/occurrences/:id/status", task.UpdateOccStatusHandler)

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
