package signal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dkeye/Voice/internal/core"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

func (ctl *SignalWSController) writePump(ctx context.Context, c *WsSignalConn) {
	for {
		select {
		case <-ctx.Done():
			log.Info().Str("module", "signal").Msg("writePump ctx done")
			return
		case data, ok := <-c.send:
			if !ok {
				log.Warn().Str("module", "signal").Msg("writePump channel closed")
				return
			}
			if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				log.Error().Err(err).Str("module", "signal").Msg("writePump set deadline")
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Error().Err(err).Str("module", "signal").Msg("writePump write error")
				return
			}
		}
	}
}

func (ctl *SignalWSController) readPump(ctx context.Context, sid core.SessionID, c *WsSignalConn) {
	defer func() {
		log.Info().Str("module", "signal").Str("sid", string(sid)).Msg("readPump closing")
		c.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			log.Info().Str("module", "signal").Str("sid", string(sid)).Msg("readPump ctx done")
			return
		default:
			_, data, err := c.conn.ReadMessage()
			if err != nil {
				log.Error().Err(err).Str("module", "signal").Str("sid", string(sid)).Msg("readPump read error")
				return
			}
			ctl.handleSignal(sid, c, data)
		}
	}
}

func (ctl *SignalWSController) handleSignal(sid core.SessionID, c *WsSignalConn, data []byte) {
	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("bad json")
		return
	}

	switch env.Type {
	case "join":
		ctl.handleJoin(sid, c, data)
	case "leave":
		ctl.handleLeave(sid, c)
	case "ping":
		ctl.handlePing(c)
	case "rename":
		ctl.handleRename(sid, c, data)
	case "whoami":
		ctl.handleWhoAmI(sid, c)
	case "offer":
		ctl.handleOffer(sid, c, data)
	case "answer":
		ctl.handleAnswer(sid, c, data)
	case "candidate":
		ctl.handleCandidate(sid, c, data)
	default:
		log.Warn().Str("module", "signal").Str("type", env.Type).Msg("unknown signal")
	}
}

func (ctl *SignalWSController) sendJSON(c *WsSignalConn, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("sendJSON marshal")
		return
	}
	_ = c.TrySend(b)
}
