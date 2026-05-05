package auth

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
	"github.com/BarisNKorkmaz/taskManager/middleware"
	"github.com/BarisNKorkmaz/taskManager/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var testApp *fiber.App
var testUser User

func TestMain(m *testing.M) {
	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Printf("Warning: .env file not found: %v\n", err)
	}

	// Set required env vars for tests
	os.Setenv("JWT_SECRET", "test_secret")
	os.Setenv("FRONTEND_URL", "http://localhost:3000")
	os.Setenv("MAIL_PORT", "2525")
	os.Setenv("MAIL_HOST", "localhost")
	os.Setenv("MAIL_USER", "test")
	os.Setenv("MAIL_PASS", "test")
	os.Setenv("MAIL_FROM", "test@test.com")

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
	database.DB.Migrator().DropTable(&User{}, &Session{}, &PasswordResetToken{}, &deviceTokenInterface{})

	if err := database.DB.AutoMigrate(&User{}, &Session{}, &PasswordResetToken{}, &deviceTokenInterface{}); err != nil {
		return err
	}

	hashedPass, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	testUser = User{
		Name:          "Test",
		Surname:       "User",
		Email:         "testauth@example.com",
		PasswordHash:  string(hashedPass),
		PassChangedAt: time.Now(),
		Timezone:      "Europe/Istanbul",
		IsActive:      true,
	}

	if err := database.DB.Create(&testUser).Error; err != nil {
		return err
	}

	return nil
}

func setupTestApp() *fiber.App {
	app := fiber.New()

	app.Post("/auth/register", RegisterHandler)
	app.Post("/auth/login", LoginHandler)
	app.Post("/auth/refresh", RefreshHandler)
	app.Post("/auth/forgot-password", ForgotPasswordHandler)
	app.Post("/auth/reset-password", ResetPasswordHandler)

	// Protected routes
	protected := app.Group("/api", func(c fiber.Ctx) error {
		c.Locals("userId", testUser.UserID)
		c.Locals("sessionId", "test-session-id")
		return c.Next()
	})
	protected.Get("/auth/me", MeHandler)
	protected.Post("/auth/logout", LogoutHandler)

	return app
}

func cleanupTestData() {
	if database.DB != nil {
		database.DB.Unscoped().Where("user_id = ?", testUser.UserID).Delete(&Session{})
		database.DB.Unscoped().Where("user_id = ?", testUser.UserID).Delete(&PasswordResetToken{})
		database.DB.Unscoped().Where("user_id = ?", testUser.UserID).Delete(&User{})
	}
}

func TestRegisterHandler(t *testing.T) {
	t.Run("Successful Registration", func(t *testing.T) {
		reqBody := RegisterDTO{
			Name:     "New",
			Surname:  "User",
			Email:    "newuser@example.com",
			Password: "newpassword123",
			Timezone: "Europe/Istanbul",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		if resp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 201, got %d, body: %s", resp.StatusCode, string(respBody))
		}

		// Verify user was created
		var user User
		if err := database.DB.Where("email = ?", "newuser@example.com").First(&user).Error; err != nil {
			t.Errorf("Expected to find user in DB, got error: %v", err)
		}

		// Cleanup
		database.DB.Unscoped().Where("email = ?", "newuser@example.com").Delete(&User{})
	})

	t.Run("Email Already Used", func(t *testing.T) {
		reqBody := RegisterDTO{
			Name:     "Another",
			Surname:  "User",
			Email:    testUser.Email, // Email already exists
			Password: "password123",
			Timezone: "Europe/Istanbul",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected status 409 for duplicate email, got %d", resp.StatusCode)
		}
	})

	t.Run("Invalid Payload", func(t *testing.T) {
		reqBody := map[string]string{
			"email": "not-an-email", // Invalid email format
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid payload, got %d", resp.StatusCode)
		}
	})
}

func TestLoginHandler(t *testing.T) {
	t.Run("Successful Login", func(t *testing.T) {
		reqBody := LoginDTO{
			Email:    testUser.Email,
			Password: "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d, body: %s", resp.StatusCode, string(respBody))
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if _, ok := result["accessToken"]; !ok {
			t.Errorf("Expected accessToken in response")
		}

		// Check if refresh_token cookie is set
		hasRefreshToken := false
		for _, cookie := range resp.Cookies() {
			if cookie.Name == "refresh_token" {
				hasRefreshToken = true
				break
			}
		}
		if !hasRefreshToken {
			t.Errorf("Expected refresh_token cookie to be set")
		}
	})

	t.Run("Invalid Credentials", func(t *testing.T) {
		reqBody := LoginDTO{
			Email:    testUser.Email,
			Password: "wrongpassword",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for wrong password, got %d", resp.StatusCode)
		}
	})

	t.Run("Non-existent User", func(t *testing.T) {
		reqBody := LoginDTO{
			Email:    "doesnotexist@example.com",
			Password: "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for non-existent user, got %d", resp.StatusCode)
		}
	})
}

func TestMeHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/auth/me", nil)

	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	userMap, ok := result["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user object in response")
	}

	if userMap["email"] != testUser.Email {
		t.Errorf("Expected email %s, got %v", testUser.Email, userMap["email"])
	}
}

func TestRefreshHandler(t *testing.T) {
	t.Run("Missing Token", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/auth/refresh", nil)
		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for missing token, got %d", resp.StatusCode)
		}
	})

	t.Run("Valid Token", func(t *testing.T) {
		tokens := GenerateRefreshToken(testUser.UserID, testUser.Email, "127.0.0.1")
		if tokens.err != nil {
			t.Fatalf("Failed to generate test token: %v", tokens.err)
		}

		req := httptest.NewRequest("POST", "/auth/refresh", nil)
		req.AddCookie(&http.Cookie{
			Name:  "refresh_token",
			Value: tokens.refreshToken,
		})

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if _, ok := result["accessToken"]; !ok {
			t.Errorf("Expected accessToken in response")
		}
	})
}

func TestLogoutHandler(t *testing.T) {
	tokens := GenerateRefreshToken(testUser.UserID, testUser.Email, "127.0.0.1")
	
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: tokens.refreshToken,
	})

	resp, err := testApp.Test(req)
	if err != nil {
		t.Fatalf("Failed to test app: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify cookie is cleared (has empty value or past expiration)
	cleared := false
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "refresh_token" {
			if cookie.Value == "" || cookie.Expires.Before(time.Now()) {
				cleared = true
			}
		}
	}

	if !cleared {
		t.Errorf("Expected refresh_token cookie to be cleared")
	}
}

func TestForgotPasswordHandler(t *testing.T) {
	t.Run("Valid Email", func(t *testing.T) {
		reqBody := ForgotPassDTO{
			Email: testUser.Email,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/forgot-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Non-existent Email", func(t *testing.T) {
		reqBody := ForgotPassDTO{
			Email: "doesnotexist@example.com",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/forgot-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 even for non-existent email to prevent enumeration, got %d", resp.StatusCode)
		}
	})
}

func TestResetPasswordHandler(t *testing.T) {
	// First create a reset token
	token, hashedToken, err := utils.GeneratePassResetToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	resetToken := PasswordResetToken{
		UserID:    testUser.UserID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	database.DB.Create(&resetToken)

	t.Run("Successful Reset", func(t *testing.T) {
		reqBody := ResetPassDTO{
			Token:           token,
			NewPassword:     "newpassword123",
			ConfirmPassword: "newpassword123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/reset-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := testApp.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d, body: %s", resp.StatusCode, string(respBody))
		}

		// Verify token was deleted
		var count int64
		database.DB.Model(&PasswordResetToken{}).Where("id = ?", resetToken.ID).Count(&count)
		if count != 0 {
			t.Errorf("Expected reset token to be deleted")
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		reqBody := ResetPassDTO{
			Token:           "invalid_token",
			NewPassword:     "newpassword123",
			ConfirmPassword: "newpassword123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/auth/reset-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := testApp.Test(req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid token, got %d", resp.StatusCode)
		}
	})
}
