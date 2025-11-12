package adapters

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type SignalWSController struct {
	Orch *app.Orchestrator
}

type wsSignalConn struct {
	conn *websocket.Conn
	send chan core.Frame
	once sync.Once
}

func (c *wsSignalConn) TrySend(f core.Frame) error {
	select {
	case c.send <- f:
		return nil
	default:
		return ErrBackpressure
	}
}

func (c *wsSignalConn) Close() {
	c.once.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (ctl *SignalWSController) HandleSignal(ctx context.Context, c *gin.Context) {
	sid := core.SessionID(c.GetString("client_token"))

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}

	conn := &wsSignalConn{
		conn: ws,
		send: make(chan core.Frame, 32),
	}

	user := ctl.Orch.Registry.GetOrCreateUser(sid)
	meta := domain.NewMember(user)
	sess := core.NewMemberSession(meta, conn)
	ctx, cancel := context.WithCancel(ctx)
	ctl.Orch.Registry.BindSignal(sid, sess, cancel)

	go ctl.writePump(ctx, conn)
	go ctl.readPump(ctx, sid, conn)
}

func (ctl *SignalWSController) writePump(ctx context.Context, c *wsSignalConn) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-c.send:
				if !ok {
					return
				}
				_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
					return
				}
			}
		}
	}()
}

func (ctl *SignalWSController) readPump(ctx context.Context, sid core.SessionID, c *wsSignalConn) {
	go func() {
		defer func() {
			ctl.Orch.OnDisconnect(sid)
			c.Close()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, data, err := c.conn.ReadMessage()
				if err != nil {
					return
				}
				ctl.handleSignal(sid, c, data)
			}
		}
	}()
}

func (ctl *SignalWSController) handleSignal(sid core.SessionID, c *wsSignalConn, data []byte) {
	var msg struct {
		Type string `json:"type"`
		Room string `json:"room,omitempty"`
		Name string `json:"name,omitempty"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Println("bad json:", err)
		return
	}

	switch msg.Type {
	case "join":
		log.Println("join room:", msg.Room)
		room := domain.RoomName(msg.Room)
		ctl.Orch.Join(sid, room)
		_ = c.TrySend([]byte(`{"type":"joined","room":"` + string(room) + `"}`))
	case "leave":
		log.Println("leave room")
		ctl.Orch.KickBySID(sid)
		_ = c.TrySend([]byte(`{"type":"leaved"}`))
	case "ping":
		_ = c.TrySend([]byte(`{"type":"pong"}`))
	case "rename":
		log.Println("rename:", msg.Name)
		if msg.Name != "" {
			ctl.Orch.Registry.UpdateUsername(sid, msg.Name)
		}
	case "whoami":
		log.Println("whoami")
		if sess, ok := ctl.Orch.Registry.GetSession(sid); ok {
			name := sess.Meta().User.Username
			_ = c.TrySend([]byte(`{"type":"name", "name":` + string(name) + `}`))
		}
	default:
		log.Println("unknown signal:", msg.Type)
	}
}
