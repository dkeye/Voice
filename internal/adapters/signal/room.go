package signal

import (
	"encoding/json"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

func (ctl *SignalWSController) createRoom(
	sid core.SessionID,
	conn *WsSignalConn,
	data []byte,
) {
	roomID, _, ok := ctl.Orch.Registry.RoomOf(sid)
	if ok {
		if _, ok := ctl.Orch.Rooms.GetRoom(roomID); ok {
			ctl.sendJSON(conn, map[string]any{
				"type":  "error",
				"error": "you alredy in room",
			})
			return
		}
	}
	type Payload struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("bad join payload")
		ctl.sendJSON(conn, map[string]any{
			"type":  "error",
			"error": "bad_payload",
		})
		return
	}
	raw := p.Name
	if len(raw) > 36 {
		raw = raw[:36] // TODO: change to setter
	}
	name := domain.RoomName(raw)

	room := ctl.Orch.Rooms.CreateRoom(name)
	resp := struct {
		Type string `json:"type"`
		Room string `json:"room"`
	}{
		"room_created",
		string(room.Room().ID),
	}
	ctl.sendJSON(conn, resp)
}

func (ctl *SignalWSController) handleJoin(
	sid core.SessionID,
	conn *WsSignalConn,
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
		ctl.sendJSON(conn, map[string]any{
			"type":  "error",
			"error": "bad_payload",
		})
		return
	}
	room, ok := ctl.Orch.Rooms.GetRoom(domain.RoomID(p.Room))
	if !ok {
		log.Error().Str("module", "signal").Str("room_id", p.Room).Msg("room is not exists")
		ctl.sendJSON(conn, map[string]any{
			"type":  "error",
			"error": "room is not exists",
		})
		return
	}

	if p.Name != "" {
		ctl.Orch.Registry.UpdateUsername(sid, p.Name)
		log.Info().Str("module", "signal").Str("sid", string(sid)).Str("name", p.Name).Msg("rename on join")
	}

	log.Info().Str("module", "signal").Str("sid", string(sid)).Str("room_id", string(p.Room)).Msg("join")
	ctl.Orch.Join(sid, domain.RoomID(p.Room))
	clientResp := struct {
		Type     string           `json:"type"`
		Room     domain.RoomID    `json:"room"`
		RoomName domain.RoomName  `json:"room_name"`
		Members  []core.MemberDTO `json:"members"`
		Count    int              `json:"count"`
	}{
		Type:     "room_state",
		Room:     room.Room().ID,
		RoomName: room.Room().Name,
		Members:  room.MembersSnapshot(),
		Count:    room.MemberCount(),
	}
	ctl.sendJSON(conn, clientResp)

	user, _ := ctl.Orch.Registry.GetOrCreateUser(sid)

	broadcastResp := struct {
		Type string      `json:"type"`
		User domain.User `json:"user"`
	}{
		Type: "member_joined",
		User: *user,
	}
	ctl.BroadcastFrom(sid, broadcastResp)
}

// handleLeave — выход из текущей комнаты, соединение при этом не рвётся.
func (ctl *SignalWSController) handleLeave(
	sid core.SessionID,
	conn *WsSignalConn,
) {
	log.Info().Str("module", "signal").Str("sid", string(sid)).Msg("leave")
	roomID, _, ok := ctl.Orch.Registry.RoomOf(sid)

	ctl.Orch.KickBySID(sid)
	ctl.sendJSON(conn, map[string]any{
		"type": "left",
	})

	if ok {
		user, _ := ctl.Orch.Registry.GetOrCreateUser(sid)

		broadcastResp := struct {
			Type string      `json:"type"`
			User domain.User `json:"user"`
		}{
			Type: "member_left",
			User: *user,
		}
		ctl.BroadcastRoom(roomID, broadcastResp)
	}
}
