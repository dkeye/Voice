package http

import (
	"fmt"
	"net/http"

	"github.com/dkeye/Voice/internal/core"
	"github.com/gin-gonic/gin"
)

type JoinRequest struct {
	Name string `json:"name"`
}

type JoinResponse struct {
	Message string `json:"message"`
}

func SetupRouter() *gin.Engine {
	router := gin.Default()
	room := core.NewRoom("main")

	router.POST("/join", handleJoinRoom(room))

	return router
}

func handleJoinRoom(r *core.Room) func(c *gin.Context) {
	return func(c *gin.Context) {
		var req JoinRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid name"})
			return
		}
		client := core.NewClient(req.Name)
		client.Room = r
		r.AddClient(client)

		c.JSON(http.StatusOK, JoinResponse{
			Message: fmt.Sprintf("Client %s join room %s!", client.Name, r.Name),
		})
	}
}
