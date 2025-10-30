package http

import (
	"context"
	"log"
	"net/http"

	"github.com/dkeye/Voice/internal/config"
	"github.com/dkeye/Voice/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type JoinRequest struct {
	Name string `json:"name"`
}

type JoinResponse struct {
	Message string `json:"message"`
}

func SetupRouter(ctx context.Context, cfg *config.Config) *gin.Engine {
	r := gin.New()
	if cfg.Mode == "debug" {
		r.Use(gin.Logger())
	}
	r.Use(gin.Recovery())

	r.Static("/static", cfg.StaticPath)
	r.GET("/", func(c *gin.Context) {
		c.File(cfg.StaticPath + "/index.html")
	})

	rm := core.NewRoomManager(ctx)

	r.GET("/join", handleJoinRoom(rm))
	r.GET("/rooms", func(c *gin.Context) {
		c.JSON(200, gin.H{"rooms": rm.List()})
	})
	return r
}

func handleJoinRoom(rm *core.RoomManager) func(c *gin.Context) {
	return func(c *gin.Context) {
		username := c.Query("name")
		roomName := c.Query("room")
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing name"})
			return
		}
		if roomName == "" {
			roomName = "main"
		}

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("upgrade error:", err)
			return
		}
		room := rm.GetOrCreate(roomName)

		client := core.NewClient(username, room, conn)
		room.AddClient(client)

		go client.ReadPump()
		go client.WritePump()
	}
}
