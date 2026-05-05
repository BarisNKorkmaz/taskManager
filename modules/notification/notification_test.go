package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var testApp *fiber.App

func TestMain(m *testing.M) {
	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Printf("Warning: .env file not found: %v\n", err)
	}

	middleware.Init()

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
	database.DB.Migrator().DropTable(&DeviceToken{})

	if err := database.DB.AutoMigrate(&DeviceToken{}); err != nil {
		return err
	}

	return nil
}

func setupTestApp() *fiber.App {
	app := fiber.New()

	protected := app.Group("/api", func(c fiber.Ctx) error {
		c.Locals("userId", uint(1))
		c.Locals("sessionId", "test-session-id")
		return c.Next()
	})

	protected.Post("/notifications/register", RegisterPushTokenHandler)
	protected.Delete("/notifications/delete", DeletePushTokenHandler)
	protected.Post("/notifications/test", TestPushHandler)

	return app
}

func cleanupTestData() {
	if database.DB != nil {
		database.DB.Unscoped().Where("user_id = ?", uint(1)).Delete(&DeviceToken{})
	}
}

func TestRegisterPushTokenHandler(t *testing.T) {
	// Clean DB before test
	database.DB.Unscoped().Where("1=1").Delete(&DeviceToken{})

	t.Run("Successful Registration", func(t *testing.T) {
		appVersion := "1.0.0"
		deviceId := "test-device-id"
		reqBody := RegisterPushTokenDTO{
			Token:      "test-fcm-token-which-is-at-least-20-chars",
			Platform:   "ios",
			AppVersion: &appVersion,
			DeviceID:   &deviceId,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/notifications/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		if resp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 201, got %d, body: %s", resp.StatusCode, string(respBody))
		}

		// Verify token was created
		var token DeviceToken
		if err := database.DB.Where("token = ?", reqBody.Token).First(&token).Error; err != nil {
			t.Errorf("Expected to find token in DB, got error: %v", err)
		}
	})

	t.Run("Update Existing Token", func(t *testing.T) {
		appVersion := "1.0.1"
		deviceId := "test-device-id"
		reqBody := RegisterPushTokenDTO{
			Token:      "test-fcm-token-which-is-at-least-20-chars",
			Platform:   "ios",
			AppVersion: &appVersion,
			DeviceID:   &deviceId,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/notifications/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		// Handler returns 200 OK for updating an existing token
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for updating existing token, got %d", resp.StatusCode)
		}

		var token DeviceToken
		database.DB.Where("token = ?", reqBody.Token).First(&token)
		if *token.AppVersion != "1.0.1" {
			t.Errorf("Expected app version to be updated to 1.0.1, got %s", *token.AppVersion)
		}
	})

	t.Run("Invalid Payload", func(t *testing.T) {
		reqBody := RegisterPushTokenDTO{
			Token:    "short-token", // Should fail min=20 validation
			Platform: "windows",     // Should fail oneof validation
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/notifications/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid payload, got %d", resp.StatusCode)
		}
	})
}

func TestDeletePushTokenHandler(t *testing.T) {
	// Setup: insert a token
	database.DB.Unscoped().Where("1=1").Delete(&DeviceToken{})
	tokenStr := "test-fcm-token-which-is-at-least-20-chars"
	token := DeviceToken{
		UserID:    1,
		SessionID: "test-session-id",
		Token:     tokenStr,
		Platform:  "android",
		IsActive:  true,
	}
	database.DB.Create(&token)

	t.Run("Successful Deletion", func(t *testing.T) {
		reqBody := DeletePushTokenDTO{
			Token: tokenStr,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("DELETE", "/api/notifications/delete", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d, body: %s", resp.StatusCode, string(respBody))
		}

		// Verify token is deactivated
		var deletedToken DeviceToken
		database.DB.Where("token = ?", tokenStr).First(&deletedToken)
		if deletedToken.IsActive {
			t.Errorf("Expected token to be inactive after deletion")
		}
	})

	t.Run("Invalid Payload", func(t *testing.T) {
		reqBody := DeletePushTokenDTO{
			Token: "short", // Fails validation
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("DELETE", "/api/notifications/delete", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid payload, got %d", resp.StatusCode)
		}
	})
}

func TestTestPushHandler(t *testing.T) {
	// Setup: insert a token
	database.DB.Unscoped().Where("1=1").Delete(&DeviceToken{})
	tokenStr := "test-fcm-token-which-is-at-least-20-chars"
	token := DeviceToken{
		UserID:    1,
		SessionID: "test-session-id",
		Token:     tokenStr,
		Platform:  "web",
		IsActive:  true,
	}
	database.DB.Create(&token)

	t.Run("Send Push with Nil Client", func(t *testing.T) {
		// MessagingClient is nil by default in test environment
		req := httptest.NewRequest("POST", "/api/notifications/test", nil)
		req.Header.Set("Content-Type", "application/json")

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		// It should hit the 500 error because of nil client
		if resp.StatusCode != http.StatusInternalServerError {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 500 because messaging client is not initialized, got %d, body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("No Active Tokens Found", func(t *testing.T) {
		// Delete the token to cause a record not found error when searching for token to send to
		database.DB.Unscoped().Where("1=1").Delete(&DeviceToken{})

		req := httptest.NewRequest("POST", "/api/notifications/test", nil)
		req.Header.Set("Content-Type", "application/json")

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		// FetchDeviceTokenByUserId returns 500 when error happens. Wait, error in FetchDeviceTokenByUserId triggers a 500, but actually gorm.ErrRecordNotFound in TestPushHandler
		// Let's see the implementation.
		// if tx := database.FetchDeviceTokenByUserId(uid, &DeviceToken{}, deviceToken); tx.Error != nil { return 500 }
		// Wait, if it's gorm.ErrRecordNotFound, does FetchDeviceTokenByUserId return it as error? Yes. So it returns 500.

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 500 when no tokens found, got %d", resp.StatusCode)
		}
	})
}
