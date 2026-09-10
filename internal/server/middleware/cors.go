package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const productionOrigin = "https://yellowbird.amaanworks.me"

var allowedOrigins = []string{
	productionOrigin,
	"http://localhost",
	"http://localhost:3000",
	"http://localhost:3001",
	"http://localhost:5173",
	"http://127.0.0.1",
	"http://127.0.0.1:3000",
	"http://127.0.0.1:5173",
	"https://localhost",
	"https://localhost:3000",
	"https://localhost:3001",
	"https://localhost:5173",
	"https://127.0.0.1",
	"https://127.0.0.1:3000",
	"https://127.0.0.1:5173",
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		if !isAllowedOrigin(origin) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, Accept, X-Requested-With")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func isAllowedOrigin(origin string) bool {
	for _, allowedOrigin := range allowedOrigins {
		if strings.EqualFold(origin, allowedOrigin) {
			return true
		}
	}

	return false
}
