package adapters

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/config"
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func genClientToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func ClientTokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie("ct")
		if token == "" {
			token = genClientToken()
			c.SetCookie("ct", token, 3600*24*7, "/", "", false, true)
		}
		c.Set("client_token", token)
		c.Next()
	}
}

func SetupRouter(ctx context.Context, cfg *config.Config, orch *app.Orchestrator) *gin.Engine {
	if cfg.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	if cfg.Mode == "debug" {
		r.Use(gin.Logger())
	}
	r.Use(gin.Recovery())

	store := cookie.NewStore([]byte(cfg.Secret))
	r.Use(sessions.Sessions("VoiceSessions", store))
	r.Use(ClientTokenMiddleware())

	r.Static("/static", cfg.StaticPath)
	r.GET("/", func(c *gin.Context) {
		c.File(cfg.StaticPath + "/index.html")
	})

	log.Info().Str("module", "adapters.http").Str("static", cfg.StaticPath).Msg("router setup")

	api := r.Group("/api")

	api.GET("/rooms", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"rooms": orch.Rooms.List()})
	})

	api.GET("/rooms/:name", func(c *gin.Context) {
		name := domain.RoomName(c.Param("name"))
		room := orch.Rooms.GetOrCreate(name)
		c.JSON(http.StatusOK, gin.H{
			"name":        room.Room().Name,
			"memberCount": room.MemberCount(),
			"members":     room.MembersSnapshot(),
		})
	})

	api.POST("/me/leave", func(c *gin.Context) {
		sessionID := core.SessionID(c.GetString("client_token"))
		orch.KickBySID(sessionID)
		c.Status(http.StatusNoContent)
	})

	api.POST("/me/move", func(c *gin.Context) {
		sessionID := core.SessionID(c.GetString("client_token"))
		to := c.Query("to")
		if to == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing to"})
			return
		}
		ok := orch.Move(sessionID, to)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	// api.GET("/ws/join", func(c *gin.Context) {
	// 	username := c.Query("name")
	// 	if username == "" {
	// 		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id or name"})
	// 		return
	// 	}
	// 	sessionID := core.SessionID(c.GetString("client_token"))
	// 	userID := domain.UserID(c.GetString("client_token"))
	// 	roomName := domain.RoomName(c.DefaultQuery("room", "main"))

	// 	upgrader := websocket.Upgrader{
	// 		CheckOrigin: func(r *http.Request) bool { return true },
	// 	}
	// 	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	// 	if err != nil {
	// 		log.Println("websocket upgrade error:", err)
	// 		return
	// 	}

	// 	user := domain.NewUser(userID, username)
	// 	memberMeta := domain.NewMember(user)
	// 	wsConn := NewWSConnection(sessionID, ws)
	// 	session := core.NewMemberSession(memberMeta, wsConn)
	// 	connCtx, connCancel := context.WithCancel(ctx)

	// 	orch.Join(sessionID, roomName, session, connCancel)

	// 	wsConn.StartWriteLoop(connCtx)
	// 	wsConn.StartReadLoop(connCtx, orch)
	// })

	api.GET("/ws/signal", func(c *gin.Context) {
		ctrl := &SignalWSController{
			Orch: orch,
		}
		log.Info().Str("module", "adapters.http").Str("sid", c.GetString("client_token")).Msg("ws signal endpoint hit")
		ctrl.HandleSignal(ctx, c)
	})

	return r
}
