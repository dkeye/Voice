package signal

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/dkeye/Voice/internal/app/orch"
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var ErrBackpressure = errors.New("backpressure")

type SignalWSController struct {
	Orch *orch.Orchestrator
}

func NewSignalWSController(orch orch.Orchestrator) *SignalWSController {
	return &SignalWSController{
		Orch: &orch,
	}
}

type WsSignalConn struct {
	conn *websocket.Conn
	send chan core.Frame

	mu     sync.RWMutex
	closed bool
}

func (c *WsSignalConn) TrySend(f core.Frame) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return errors.New("connection closed")
	}
	select {
	case c.send <- f:
	default:
		return ErrBackpressure
	}
	return nil
}

func (c *WsSignalConn) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.send)
	_ = c.conn.Close()
	c.mu.Unlock()
}

func (ctl *SignalWSController) BroadcastFrom(sid core.SessionID, v any) {
	for _, roomMate := range ctl.Orch.Registry.RoomMates(sid) {
		ctl.sendJSON(roomMate.Session.Signal(), v)
	}
}

func (ctl *SignalWSController) BroadcastRoom(roomID domain.RoomID, v any) {
	for _, snap := range ctl.Orch.Registry.MembersOfRoom(roomID) {
		ctl.sendJSON(snap.Session.Signal(), v)
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (ctl *SignalWSController) HandleSignal(ctx context.Context, c *gin.Context) {
	sid := core.SessionID(c.GetString("client_token"))
	log.Info().Str("module", "signal").Str("sid", string(sid)).Msg("new WS connection")

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("ws upgrade")
		return
	}

	conn := &WsSignalConn{
		conn: ws,
		send: make(chan core.Frame, 32),
	}

	user, _ := ctl.Orch.Registry.GetOrCreateUser(sid)
	meta := domain.NewMember(user)
	sess := core.NewMemberSession(meta).UpdateSignal(conn)
	ctx, cancel := context.WithCancel(ctx)
	ctl.Orch.Registry.BindSignal(sid, sess, cancel)

	go ctl.writePump(ctx, conn)
	go ctl.readPump(ctx, sid, conn)
}
