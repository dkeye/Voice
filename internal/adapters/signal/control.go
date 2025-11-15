package signal

import (
	"encoding/json"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

func (ctl *SignalWSController) handleJoin(
	sid core.SessionID,
	conn *wsSignalConn,
	data []byte,
) {
	type joinPayload struct {
		Type string `json:"type"`
		Room string `json:"room"`
		Name string `json:"name,omitempty"`
	}
	var p joinPayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("bad join payload")
		return
	}

	roomName := domain.RoomName(p.Room)
	if roomName == "" {
		roomName = "main"
	}

	if p.Name != "" {
		ctl.Orch.Registry.UpdateUsername(sid, p.Name)
		log.Info().Str("module", "signal").Str("sid", string(sid)).Str("name", p.Name).Msg("rename on join")
	}

	log.Info().Str("module", "signal").Str("sid", string(sid)).Str("room", string(roomName)).Msg("join")
	ctl.Orch.Join(sid, roomName)

	// Отдаём снапшот комнаты, чтобы клиент мог обновить UI.
	room := ctl.Orch.Rooms.GetOrCreate(roomName)
	resp := struct {
		Type    string           `json:"type"`
		Room    domain.RoomName  `json:"room"`
		Members []core.MemberDTO `json:"members"`
		Count   int              `json:"count"`
	}{
		Type:    "room_state",
		Room:    room.Room().Name,
		Members: room.MembersSnapshot(),
		Count:   room.MemberCount(),
	}
	ctl.sendJSON(conn, resp)
}

// handleLeave — выход из текущей комнаты, соединение при этом не рвётся.
func (ctl *SignalWSController) handleLeave(
	sid core.SessionID,
	conn *wsSignalConn,
) {
	log.Info().Str("module", "signal").Str("sid", string(sid)).Msg("leave")
	ctl.Orch.KickBySID(sid)
	ctl.sendJSON(conn, map[string]any{
		"type": "left",
	})
}

func (ctl *SignalWSController) handlePing(
	conn *wsSignalConn,
) {
	resp := struct {
		Type string `json:"type"`
	}{
		Type: "pong",
	}
	ctl.sendJSON(conn, resp)
}

func (ctl *SignalWSController) handleRename(
	sid core.SessionID,
	conn *wsSignalConn,
	data []byte,
) {
	type renamePayload struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	var p renamePayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("bad rename payload")
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
	ctl.handleWhoAmI(sid, conn)
}

func (ctl *SignalWSController) handleWhoAmI(
	sid core.SessionID,
	conn *wsSignalConn,
) {
	user := ctl.Orch.Registry.GetOrCreateUser(sid)
	roomName, _, ok := ctl.Orch.Registry.RoomOf(sid)

	resp := struct {
		Type     string          `json:"type"`
		Username string          `json:"username"`
		Room     domain.RoomName `json:"room,omitempty"`
	}{
		Type:     "whoami",
		Username: user.Username,
	}
	if ok {
		resp.Room = roomName
	}
	ctl.sendJSON(conn, resp)
}
