package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/dkeye/Voice/internal/adapters/signal"
	"github.com/dkeye/Voice/internal/app/orch"
	"github.com/dkeye/Voice/internal/config"
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

func SetupRouter(ctx context.Context, cfg *config.Config, orch *orch.Orchestrator) *gin.Engine {
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

	api.GET("/ws/signal", func(c *gin.Context) {
		ctrl := &signal.SignalWSController{
			Orch: orch,
		}
		log.Info().Str("module", "adapters.http").Str("sid", c.GetString("client_token")).Msg("ws signal endpoint hit")
		ctrl.HandleSignal(ctx, c)
	})

	return r
}
