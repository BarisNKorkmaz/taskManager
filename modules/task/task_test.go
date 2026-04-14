package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/modules/auth"
	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var testApp *fiber.App
var testUser auth.User

func TestMain(m *testing.M) {
	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Printf("Warning: .env file not found: %v\n", err)
	}

	if err := setupTestDB(); err != nil {
		fmt.Printf("Failed to setup test database: %v\n", err)
		os.Exit(1)
	}

	testApp = setupTestApp()

	code := m.Run()

	cleanupTestData()

	os.Exit(code)
}

func setupTestDB() error {
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")
	sslMode := os.Getenv("DB_SSLMODE")

	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", host, user, pass, dbname, port, sslMode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	database.DB = db

	testUser = auth.User{
		Name:          "Test",
		Surname:       "User",
		Email:         "testuser@example.com",
		PasswordHash:  "hashedpassword",
		PassChangedAt: time.Now(),
		Timezone:      "Europe/Istanbul",
	}

	database.DB.Unscoped().Where("email = ?", testUser.Email).Delete(&auth.User{})

	if err := database.DB.Create(&testUser).Error; err != nil {
		return err
	}

	if err := database.DB.AutoMigrate(&auth.User{}, &TaskTemplate{}, &TaskOccurrence{}); err != nil {
		return err
	}

	return nil
}

func setupTestApp() *fiber.App {
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		c.Locals("userId", testUser.UserID)
		return c.Next()
	})

	app.Post("/tasks/templates", CreateTaskTemplateHandler)
	app.Get("/tasks/templates", GetTaskTemplatesHandler)
	app.Get("/tasks/templates/:id", GetTemplateDetailHandler)
	app.Patch("/tasks/templates/:id", UpdateTaskTemplateHandler)
	app.Patch("/tasks/templates/:id/status", SetTaskTemplateStatusHandler)
	app.Get("/dashboard", DashboardHandler)
	app.Get("/tasks/today", GetTodayOccs)
	app.Patch("/tasks/occurrences/:id/status", UpdateOccStatusHandler)

	return app
}

func cleanupTestData() {
	if database.DB != nil {
		database.DB.Unscoped().Where("user_id = ?", testUser.UserID).Delete(&TaskOccurrence{})
		database.DB.Unscoped().Where("user_id = ?", testUser.UserID).Delete(&TaskTemplate{})
		database.DB.Unscoped().Where("user_id = ?", testUser.UserID).Delete(&auth.User{})
	}
}

func TestGetLocalDate(t *testing.T) {
	user := auth.User{Timezone: "Europe/Istanbul"}
	date := getLocalDate(user)
	if date == nil {
		t.Fatal("Expected date to be not nil")
	}

	loc, _ := time.LoadLocation("Europe/Istanbul")
	now := time.Now().In(loc)
	if date.Year() != now.Year() || date.Month() != now.Month() || date.Day() != now.Day() {
		t.Errorf("Expected date %v, got %v", now.Format("2006-01-02"), date.Format("2006-01-02"))
	}
}

func TestStructToUpdateMap(t *testing.T) {
	title := "New Title"
	dto := UpdateTemplateDTO{
		Title: &title,
	}
	updates := StructToUpdateMap(dto)
	if updates["title"] != title {
		t.Errorf("Expected title %s, got %v", title, updates["title"])
	}
}

func TestNormalizeToUserDate(t *testing.T) {
	tm := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	normalized, err := NormalizeToUserDate(tm, "Europe/Istanbul")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if normalized == nil {
		t.Fatal("Expected normalized date to be not nil")
	}
	if normalized.Year() != 2026 || normalized.Month() != 4 || normalized.Day() != 14 {
		t.Errorf("Expected 2026-04-14, got %v", normalized.Format("2006-01-02"))
	}
}

func TestCreateTaskTemplateHandler(t *testing.T) {
	dt := time.Now().AddDate(0, 0, 1)
	taskReq := TaskDTO{
		Title:           "Integration Test Task",
		Description:     "Testing handler",
		IsRepeatEnabled: false,
		DueDate:         &dt,
	}

	body, _ := json.Marshal(taskReq)
	req := httptest.NewRequest("POST", "/tasks/templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
		respBody, _ := io.ReadAll(resp.Body)
		t.Logf("Response body: %s", string(respBody))
	}
}

func TestGetTaskTemplatesHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/tasks/templates", nil)
	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["activeTemplates"]; !ok {
		t.Errorf("Expected activeTemplates in response")
	}
}

func TestDashboardHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/dashboard", nil)
	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestGenerateOcc(t *testing.T) {
	repeatUnit := "day"
	repeatInterval := 1
	start := time.Now()
	template := TaskTemplate{
		ID:              100,
		UserID:          testUser.UserID,
		Title:           "Recurring Task",
		IsRepeatEnabled: true,
		RepeatUnit:      &repeatUnit,
		RepeatInterval:  &repeatInterval,
		StartDate:       &start,
	}

	templates := []TaskTemplate{template}
	now := time.Now()
	monthEnd := now.AddDate(0, 1, 0)

	occMap, err := generateOcc(templates, testUser.UserID, now, monthEnd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(occMap) == 0 {
		t.Errorf("Expected at least one occurrence in map")
	}
}

func TestUpdateOccStatusHandler(t *testing.T) {
	dt := time.Now().AddDate(0, 0, 1)
	occ := TaskOccurrence{
		TaskID:  1,
		UserID:  testUser.UserID,
		DueDate: dt,
		Status:  "pending",
	}
	database.DB.Create(&occ)

	action := TaskActionDTO{Action: "complete"}
	body, _ := json.Marshal(action)
	url := fmt.Sprintf("/tasks/occurrences/%d/status", occ.ID)
	req := httptest.NewRequest("PATCH", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "completed" {
		t.Errorf("Expected status completed, got %v", result["status"])
	}
}

func TestUpdateTaskTemplateHandler(t *testing.T) {

	template := TaskTemplate{
		UserID: testUser.UserID,
		Title:  "Original Title",
	}
	database.DB.Create(&template)

	newTitle := "Updated Title"
	updateReq := UpdateTemplateDTO{
		Title: &newTitle,
	}
	body, _ := json.Marshal(updateReq)
	url := fmt.Sprintf("/tasks/templates/%d", template.ID)
	req := httptest.NewRequest("PATCH", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestGetTemplateDetailHandler(t *testing.T) {
	template := TaskTemplate{
		UserID: testUser.UserID,
		Title:  "Detail Test Task",
	}
	database.DB.Create(&template)

	url := fmt.Sprintf("/tasks/templates/%d", template.ID)
	req := httptest.NewRequest("GET", url, nil)
	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestSetTaskTemplateStatusHandler(t *testing.T) {

	template := TaskTemplate{
		UserID:   testUser.UserID,
		Title:    "Status Toggle Task",
		IsActive: true,
	}
	database.DB.Create(&template)

	isActive := false
	statusReq := SetTemplateStatusDTO{IsActive: &isActive}
	body, _ := json.Marshal(statusReq)
	url := fmt.Sprintf("/tasks/templates/%d/status", template.ID)
	req := httptest.NewRequest("PATCH", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestGetTodayOccs(t *testing.T) {
	req := httptest.NewRequest("GET", "/tasks/today", nil)
	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
