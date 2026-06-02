package main

import (
	"os"
	_ "time/tzdata"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/middleware"
	"github.com/BarisNKorkmaz/taskManager/modules/auth"
	"github.com/BarisNKorkmaz/taskManager/modules/notification"
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

	port := os.Getenv("APP_PORT")
	if port == "" {
		middleware.Log.Error("APP_PORT env not set")
		os.Exit(1)
	}

	fbCredantialPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if fbCredantialPath == "" {
		middleware.Log.Error("FIREBASE_CREDENTIALS_PATH env not set")
		os.Exit(1)
	}
	if fbErr := notification.InitFirebase(fbCredantialPath); fbErr != nil {
		middleware.Log.Error("error initializing firebase app", "err", fbErr.Error())
		os.Exit(1)
	}

	if err := database.Connect(); err != nil {
		middleware.Log.Error("Server starting operation failed:", "err", err.Error())
		os.Exit(1)
	} else {
		middleware.Log.Info("Database connected.")
	}

	if err := database.DB.AutoMigrate(&auth.User{}, &task.TaskTemplate{}, &task.TaskOccurrence{}, &auth.Session{}, &auth.PasswordResetToken{}, &notification.DeviceToken{}); err != nil {
		middleware.Log.Error("Migration error:", "err", err.Error())
	} else {
		middleware.Log.Info("Database migrated")
	}

	app := fiber.New()

	/* app := fiber.New(fiber.Config{
		TrustProxy: true,
	}) */
	/* app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:8080", // frontend adresi
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
	})) */

	app.Use(recoverer.New())
	app.Use(logger.New())

	setupRoutes(app)
	notification.StartScheduler()

	/*
		app.Get("/health", database.HealthHandler)
		authGroup := app.Group("/auth")
		authGroup.Post("/register", auth.RegisterHandler)
		authGroup.Post("/login", auth.EmailRateLimiter(), auth.LoginIPRateLimiter(), auth.LoginHandler)
		authGroup.Post("/refresh", auth.RefreshHandler)

		protected := app.Group("/u", auth.AccessTokenMiddleware())

		protected.Get("/u/devices/test-push", notification.TestPushHandler)

		protected.Get("/auth/me", auth.MeHandler)
		protected.Post("/auth/logout", auth.LogoutHandler)

		protected.Post("/devices/push-token", notification.RegisterPushTokenHandler)
		protected.Delete("/devices/push-token", notification.DeletePushTokenHandler)

		protected.Post("/tasks/templates", task.CreateTaskTemplateHandler)
		protected.Get("/tasks/templates", task.GetTaskTemplatesHandler)
		protected.Get("/tasks/templates/:id", task.GetTemplateDetailHandler)
		protected.Patch("/tasks/templates/:id", task.UpdateTaskTemplateHandler)
		protected.Patch("/tasks/templates/:id/status", task.SetTaskTemplateStatusHandler)

		protected.Get("/dashboard", task.DashboardHandler)
		protected.Get("/tasks/today", task.GetTodayOccs)

		protected.Get("/test/report", task.WeeklyReportHandler)

		protected.Patch("/tasks/occurrences/:id/status", task.UpdateOccStatusHandler) */

	middleware.Log.Info("Starting server on :" + port)
	if err := app.Listen(":" + port); err != nil {
		middleware.Log.Error("Server failed to start", "err", err.Error())
		os.Exit(1)
	}

}

func setupRoutes(app *fiber.App) {

	v1 := app.Group("/v1")
	v1.Get("/health", database.HealthHandler)

	// --- 2. Auth Group ---
	authGroup := v1.Group("/auth")
	authGroup.Post("/register", auth.RegisterHandler)
	authGroup.Post("/login", auth.EmailRateLimiter(), auth.LoginIPRateLimiter(), auth.LoginHandler)
	authGroup.Post("/refresh", auth.RefreshHandler)
	authGroup.Post("/forgot-password", auth.ForgotPasswordHandler)
	authGroup.Post("/reset-password", auth.ResetPasswordHandler)

	// --- 3. Protected Routes (JWT Required) ---
	api := v1.Group("/u", auth.AccessTokenMiddleware())

	// User
	api.Get("/me", auth.MeHandler)
	api.Post("/auth/logout", auth.LogoutHandler)

	// Notifications
	notifications := api.Group("/notifications")
	notifications.Post("/tokens", notification.RegisterPushTokenHandler)
	notifications.Delete("/tokens", notification.DeletePushTokenHandler)

	// Task Templates
	templates := api.Group("/templates")
	templates.Get("/", task.GetTaskTemplatesHandler)
	templates.Post("/", task.CreateTaskTemplateHandler)
	templates.Get("/:id", task.GetTemplateDetailHandler)
	templates.Patch("/:id", task.UpdateTaskTemplateHandler)
	templates.Patch("/:id/status", task.SetTaskTemplateStatusHandler)

	// Tasks & Dashboard
	api.Get("/dashboard", task.DashboardHandler)
	api.Get("/tasks/today", task.GetTodayOccs)
	api.Patch("/tasks/occurrences/:id", task.UpdateOccStatusHandler)

	// Reports
	api.Get("/reports/weekly", task.WeeklyReportHandler)
}
