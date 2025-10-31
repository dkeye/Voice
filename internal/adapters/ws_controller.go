package adapters

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/gorilla/websocket"
)

var ErrBackpressure = errors.New("backpressure")

// WSConn is an indirection over *websocket.Conn to ease testing.
type WSConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(mt int, data []byte) error
	SetWriteDeadline(t time.Time) error
	Close() error
}

// WSConnection is a transport endpoint (WebSocket).
// It implements core.MemberConnection.
type WSConnection struct {
	id   domain.UserID
	conn WSConn
	send chan core.Frame
	once sync.Once
}

func NewWSConnection(id domain.UserID, conn WSConn) *WSConnection {
	return &WSConnection{
		id:   id,
		conn: conn,
		send: make(chan core.Frame, 256),
	}
}

func (c *WSConnection) ID() domain.UserID { return c.id }

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

// StartWriteLoop pumps frames to the network.
// Adapter owns transport resources and closes them on exit.
func (c *WSConnection) StartWriteLoop(ctx context.Context) {
	go func() {
		defer c.Close()
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

// StartReadLoop reads frames and forwards them to orchestrator.
// IMPORTANT: On exit we must remove the member from the room to avoid leaks.
func (c *WSConnection) StartReadLoop(ctx context.Context, room core.RoomService, o *app.Orchestrator) {
	go func() {
		defer c.Close()
		defer room.RemoveMember(c.id)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, data, err := c.conn.ReadMessage()
				if err != nil {
					return
				}
				o.OnFrameReceived(room, c.id, core.Frame(data))
			}
		}
	}()
}
