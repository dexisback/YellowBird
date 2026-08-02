package middleware

//what this does : reads Authorisation: Bearer <token>
//validates the jwt
//extracts the userID
//stores it in Gin's context
//rejects unauthorised access

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dexisback/YellowBird/internal/auth"
	"github.com/gin-gonic/gin"
)

func Auth(jwtService *auth.JWTService) gin.HandlerFunc {
	fmt.Println("auth middleware hit ! 🥀🥀🥀🥀")
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")

		if header == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing auth header",
			})
			c.Abort()
			return
		}

		//yaar yhi to bkc nhi psnd split and shi wali
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid auth header",
			})
			c.Abort()
			return
		}

		claims, err := jwtService.ValidateToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token",
			})
			c.Abort()
			return
		}
		//else everything is nice and good:
		c.Set("userID", claims.UserID)
		c.Next()
	}

}
