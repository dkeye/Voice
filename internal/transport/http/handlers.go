package http

import (
	"log"
	"net/http"

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

func SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	router.Static("/static", "./web")
	router.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})

	room := core.NewRoom("main")
	go room.Run()

	router.GET("/join", handleJoinRoom(room))

	return router
}

func handleJoinRoom(r *core.Room) func(c *gin.Context) {
	return func(c *gin.Context) {
		name := c.Query("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid name"})
			return

		}

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("upgrade error:", err)
			return
		}

		client := core.NewClient(name, r, conn)
		r.Register <- client

		go client.ReadPump()
		go client.WritePump()
	}
}
