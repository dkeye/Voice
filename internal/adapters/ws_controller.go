package adapters

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/core"
	"github.com/gorilla/websocket"
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
		return ErrBackpressure
	}
}

func (c *WSConnection) Close() {
	c.once.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

func (c *WSConnection) StartWriteLoop(ctx context.Context) {
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
				if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
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
				return
			default:
				_, data, err := c.conn.ReadMessage()
				if err != nil {
					return
				}
				o.OnFrame(c.sid, core.Frame(data))
			}
		}
	}()
}
