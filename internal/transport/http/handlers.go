package http

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type NickRequest struct {
	Name string `json:"name"`
}

type NickResponse struct {
	Message string `json:"message"`
}

func SetupRouter() *gin.Engine {
	router := gin.Default()

	router.POST("/echo", handlerEcho)

	return router
}

func handlerEcho(c *gin.Context) {
	var req NickRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid name"})
		return
	}

	c.JSON(http.StatusOK, NickResponse{
		Message: fmt.Sprintf("Hello %s!", req.Name),
	})
}
