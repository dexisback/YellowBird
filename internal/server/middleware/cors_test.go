package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowedOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		origin string
	}{
		{name: "production origin", origin: productionOrigin},
		{name: "localhost development", origin: "http://localhost:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(CORS())
			router.GET("/", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", tt.origin)
			res := httptest.NewRecorder()

			router.ServeHTTP(res, req)

			if res.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
			}

			if got := res.Header().Get("Access-Control-Allow-Origin"); got != tt.origin {
				t.Fatalf("expected allow origin %q, got %q", tt.origin, got)
			}

			if got := res.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
				t.Fatalf("expected allow credentials true, got %q", got)
			}

			if got := res.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
				t.Fatalf("unexpected allow methods header %q", got)
			}
		})
	}
}

func TestCORSRejectsDisallowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORS())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.example")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORS())
	router.OPTIONS("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", productionOrigin)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != productionOrigin {
		t.Fatalf("expected allow origin %q, got %q", productionOrigin, got)
	}

	if got := res.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("expected allow headers header to be populated")
	}
}
