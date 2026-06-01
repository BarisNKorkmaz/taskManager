package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/BarisNKorkmaz/taskManager/middleware"
	"github.com/gofiber/fiber/v3"
)

func TestMain(m *testing.M) {
	middleware.Init()
	code := m.Run()
	os.Exit(code)
}

func TestSetupRoutes(t *testing.T) {
	app := fiber.New()
	setupRoutes(app)

	expectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/health"},
		{"POST", "/v1/auth/register"},
		{"POST", "/v1/auth/login"},
		{"POST", "/v1/auth/refresh"},
		{"POST", "/v1/auth/forgot-password"},
		{"POST", "/v1/auth/reset-password"},
		// Protected routes now have /u prefix
		{"GET", "/v1/u/me"},
		{"POST", "/v1/u/auth/logout"},
		{"POST", "/v1/u/notifications/tokens"},
		{"DELETE", "/v1/u/notifications/tokens"},
		{"GET", "/v1/u/templates/"},
		{"POST", "/v1/u/templates/"},
		{"GET", "/v1/u/templates/:id"},
		{"PATCH", "/v1/u/templates/:id"},
		{"PATCH", "/v1/u/templates/:id/status"},
		{"GET", "/v1/u/dashboard"},
		{"GET", "/v1/u/tasks/today"},
		{"PATCH", "/v1/u/tasks/occurrences/:id"},
		{"GET", "/v1/u/reports/weekly"},
	}

	routes := app.GetRoutes()

	for _, expected := range expectedRoutes {
		found := false
		for _, route := range routes {
			if route.Method == expected.method && route.Path == expected.path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected route %s %s was not found in registered routes", expected.method, expected.path)
		}
	}
}

func TestRouteSecurity(t *testing.T) {
	app := fiber.New()
	setupRoutes(app)

	tests := []struct {
		name        string
		method      string
		path        string
		shouldBe401 bool
	}{
		{
			name:        "Login is public",
			method:      "POST",
			path:        "/v1/auth/login",
			shouldBe401: false,
		},
		{
			name:        "Me is protected",
			method:      "GET",
			path:        "/v1/u/me",
			shouldBe401: true,
		},
		{
			name:        "Dashboard is protected",
			method:      "GET",
			path:        "/v1/u/dashboard",
			shouldBe401: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to test app: %v", err)
			}

			if tt.shouldBe401 {
				if resp.StatusCode != http.StatusUnauthorized {
					t.Errorf("Expected status 401 for protected route %s, got %d", tt.path, resp.StatusCode)
				}
			} else {
				if resp.StatusCode == http.StatusUnauthorized {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("Expected route %s to be public, but got 401 Unauthorized. Body: %s", tt.path, string(body))
				}
			}
		})
	}
}
