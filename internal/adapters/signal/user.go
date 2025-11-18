package signal

import (
	"encoding/json"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

func (ctl *SignalWSController) handleRename(
	sid core.SessionID,
	conn *WsSignalConn,
	data []byte,
) {
	type renamePayload struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	var p renamePayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("bad rename payload")
		ctl.sendJSON(conn, map[string]any{
			"type":  "error",
			"error": "bad_payload",
		})
		return
	}
	if p.Name == "" {
		ctl.sendJSON(conn, map[string]any{
			"type":  "error",
			"error": "empty name",
		})
		return
	}

	log.Info().Str("module", "signal").Str("sid", string(sid)).Str("name", p.Name).Msg("rename")
	ctl.Orch.Registry.UpdateUsername(sid, p.Name)
	if err := ctl.Orch.Registry.UpdateUsername(sid, p.Name); err != nil {
		ctl.sendJSON(conn, map[string]any{
			"type":  "error",
			"error": "invalid_name",
		})
		return
	}
	ctl.handleWhoAmI(sid, conn)
	user, _ := ctl.Orch.Registry.GetOrCreateUser(sid)

	broadcastResp := struct {
		Type string      `json:"type"`
		User domain.User `json:"user"`
	}{
		Type: "member_updated",
		User: *user,
	}
	ctl.BroadcastFrom(sid, broadcastResp)
}

func (ctl *SignalWSController) handleWhoAmI(
	sid core.SessionID,
	conn *WsSignalConn,
) {
	user, _ := ctl.Orch.Registry.GetOrCreateUser(sid)

	resp := struct {
		Type     string          `json:"type"`
		Username string          `json:"username"`
		Room     domain.RoomID   `json:"room,omitempty"`
		RoomName domain.RoomName `json:"room_name,omitempty"`
	}{
		Type:     "whoami",
		Username: user.Username,
	}
	if RoomID, _, ok := ctl.Orch.Registry.RoomOf(sid); ok {
		if room, ok := ctl.Orch.Rooms.GetRoom(RoomID); ok {
			resp.RoomName = room.Room().Name
			resp.Room = RoomID
		}
	}
	ctl.sendJSON(conn, resp)
}
