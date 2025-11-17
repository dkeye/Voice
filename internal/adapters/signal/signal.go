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
	Orch        *orch.Orchestrator
	mu          sync.RWMutex
	connections map[core.SessionID]*WsSignalConn
}

func NewSignalWSController(orch orch.Orchestrator) *SignalWSController {
	return &SignalWSController{
		Orch:        &orch,
		connections: make(map[core.SessionID]*WsSignalConn),
	}
}

type WsSignalConn struct {
	conn *websocket.Conn
	send chan core.Frame
	once sync.Once

	renegotiateMu sync.Mutex
}

func (c *WsSignalConn) TrySend(f core.Frame) error {
	select {
	case c.send <- f:
		return nil
	default:
		return ErrBackpressure
	}
}

func (c *WsSignalConn) Close() {
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
	ctl.mu.Lock()
	ctl.connections[sid] = conn
	ctl.mu.Unlock()

	user := ctl.Orch.Registry.GetOrCreateUser(sid)
	meta := domain.NewMember(user)
	sess := core.NewMemberSession(meta).UpdateSignal(conn)
	ctx, cancel := context.WithCancel(ctx)
	ctl.Orch.Registry.BindSignal(sid, sess, cancel)

	go ctl.writePump(ctx, conn)
	go ctl.readPump(ctx, sid, conn)
}
