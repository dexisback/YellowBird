//gin alr gives us recovery middleewqare (unlike chi)


package middleware


import "github.com/gin-gonic/gin"


func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}