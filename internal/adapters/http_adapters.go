package adapters

import (
	"context"
	"log"
	"net/http"

	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/config"
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// SetupRouter wires HTTP routes (REST + WS) with orchestrator and transport.
// - Static files are served from cfg.StaticPath.
// - REST is under /api/*
// - WebSocket upgrade lives at /ws/join
func SetupRouter(ctx context.Context, cfg *config.Config, orch *app.Orchestrator) *gin.Engine {
	// Use explicit mode if provided in config.
	if cfg.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	if cfg.Mode == "debug" {
		r.Use(gin.Logger())
	}
	r.Use(gin.Recovery())

	// Static UI
	r.Static("/static", cfg.StaticPath)
	r.GET("/", func(c *gin.Context) {
		c.File(cfg.StaticPath + "/index.html")
	})

	// -------------------------
	// REST API
	// -------------------------
	api := r.Group("/api")

	// GET /api/rooms — list rooms
	api.GET("/rooms", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"rooms": orch.Rooms.List()})
	})

	// POST /api/rooms — create (or get) a room
	api.POST("/rooms", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if err := c.BindJSON(&req); err != nil || req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
			return
		}
		room := orch.Rooms.GetOrCreate(domain.RoomName(req.Name))
		c.JSON(http.StatusOK, gin.H{
			"name":        room.Room().Name,
			"memberCount": room.MemberCount(),
		})
	})

	// GET /api/rooms/:name — room info
	api.GET("/rooms/:name", func(c *gin.Context) {
		name := domain.RoomName(c.Param("name"))
		room := orch.Rooms.GetOrCreate(name)
		c.JSON(http.StatusOK, gin.H{
			"name":        room.Room().Name,
			"memberCount": room.MemberCount(),
		})
	})

	// DELETE /api/rooms/:name — stop/delete room
	api.DELETE("/rooms/:name", func(c *gin.Context) {
		name := domain.RoomName(c.Param("name"))
		orch.Rooms.StopRoom(name)
		c.Status(http.StatusNoContent)
	})

	// GET /api/rooms/:name/members — list members in a room
	api.GET("/rooms/:name/members", func(c *gin.Context) {
		name := domain.RoomName(c.Param("name"))
		room := orch.Rooms.GetOrCreate(name)
		// Requires RoomService.MembersSnapshot() in core
		c.JSON(http.StatusOK, room.MembersSnapshot())
	})

	// DELETE /api/rooms/:name/members/:id — kick member
	api.DELETE("/rooms/:name/members/:id", func(c *gin.Context) {
		name := domain.RoomName(c.Param("name"))
		id := domain.UserID(c.Param("id"))
		room := orch.Rooms.GetOrCreate(name)
		// Note: this only removes membership; adapter-owned transport
		// should be closed by policy or on read-loop exit.
		room.RemoveMember(id)
		c.Status(http.StatusNoContent)
	})

	// TODO: POST /api/rooms/:name/move?id={id}&to={room}
	// Implement proper "move without reconnect" once RoomManager exposes
	// MemberSession lookup and cross-room re-attach.

	// -------------------------
	// WebSocket JOIN
	// -------------------------
	// GET /ws/join?room={roomName}&id={userId}&name={username}
	r.GET("/ws/join", func(c *gin.Context) {
		username := c.Query("name")
		idParam := c.Query("id")

		// Either id or name must be provided (id wins if both present).
		if username == "" && idParam == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing id or name"})
			return
		}
		if idParam == "" {
			idParam = username
		}
		if username == "" {
			username = idParam
		}

		userID := domain.UserID(idParam)
		roomName := domain.RoomName(c.DefaultQuery("room", "main"))

		upgrader := websocket.Upgrader{
			// TODO: In production, restrict origins as needed.
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("websocket upgrade error:", err)
			return
		}

		// Core room
		room := orch.Rooms.GetOrCreate(roomName)

		// Domain meta
		user := domain.NewUser(userID, username)
		memberMeta := domain.NewMember(user)

		// Transport endpoint
		wsConn := NewWSConnection(userID, ws)

		// Core session (meta + transport)
		session := core.NewMemberSession(memberMeta, wsConn)

		// Register in room
		room.AddMember(session)

		// Connection-scoped context; cancel inherited by server shutdown.
		connCtx, _ := context.WithCancel(ctx)

		// Start transport pumps; read-loop will remove membership on exit.
		wsConn.StartWriteLoop(connCtx)
		wsConn.StartReadLoop(connCtx, room, orch)
	})

	return r
}
