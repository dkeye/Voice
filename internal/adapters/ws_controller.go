package adapters

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/core"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var ErrBackpressure = errors.New("backpressure")

type WSConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(mt int, data []byte) error
	SetWriteDeadline(t time.Time) error
	Close() error
}

type WSConnection struct {
	sid  core.SessionID
	conn WSConn
	send chan core.Frame
	once sync.Once
}

func NewWSConnection(sid core.SessionID, conn WSConn) *WSConnection {
	return &WSConnection{
		sid:  sid,
		conn: conn,
		send: make(chan core.Frame, 256),
	}
}

func (c *WSConnection) TrySend(f core.Frame) error {
	select {
	case c.send <- f:
		return nil
	default:
		log.Warn().Str("module", "adapters.ws").Str("sid", string(c.sid)).Msg("backpressure")
		return ErrBackpressure
	}
}

func (c *WSConnection) Close() {
	c.once.Do(func() {
		close(c.send)
		if err := c.conn.Close(); err != nil {
			log.Error().Err(err).Str("module", "adapters.ws").Str("sid", string(c.sid)).Msg("close error")
		}
	})
}

func (c *WSConnection) StartWriteLoop(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info().Str("module", "adapters.ws").Str("sid", string(c.sid)).Msg("write loop ctx done")
				return
			case data, ok := <-c.send:
				if !ok {
					log.Info().Str("module", "adapters.ws").Str("sid", string(c.sid)).Msg("write loop channel closed")
					return
				}
				if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
					log.Error().Err(err).Str("module", "adapters.ws").Str("sid", string(c.sid)).Msg("set write deadline")
					return
				}
				if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					log.Error().Err(err).Str("module", "adapters.ws").Str("sid", string(c.sid)).Msg("write message")
					return
				}
			}
		}
	}()
}

func (c *WSConnection) StartReadLoop(ctx context.Context, o *app.Orchestrator) {
	go func() {
		defer func() {
			o.OnDisconnect(c.sid)
			c.Close()
		}()
		for {
			select {
			case <-ctx.Done():
				log.Info().Str("module", "adapters.ws").Str("sid", string(c.sid)).Msg("read loop ctx done")
				return
			default:
				_, data, err := c.conn.ReadMessage()
				if err != nil {
					log.Error().Err(err).Str("module", "adapters.ws").Str("sid", string(c.sid)).Msg("read error")
					return
				}
				o.OnFrame(c.sid, core.Frame(data))
			}
		}
	}()
}
