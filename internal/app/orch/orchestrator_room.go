package orch

import (
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

func (o *Orchestrator) Join(sid core.SessionID, roomID domain.RoomID) {
	existRoomID, _, ok := o.Registry.RoomOf(sid)
	if ok {
		log.Info().Str("sid", string(sid)).Str("roomID", string(existRoomID)).Msg("already in room")
		return
	}
	if session, ok := o.Registry.GetSession(sid); ok {
		room, ok := o.Rooms.GetRoom(roomID)
		if !ok {
			log.Error().Str("module", "orch").Str("room_id", string(roomID)).Msg("room not exists")
			return
		}
		room.AddMember(sid, session)
		o.Registry.UpdateRoom(sid, roomID)
		log.Info().Str("sid", string(sid)).Str("room_id", string(roomID)).Msg("added to room")
	}
}

func (o *Orchestrator) KickBySID(sid core.SessionID) {
	o.cleanupMedia(sid)
	o.cleanupMembership(sid)
}

func (o *Orchestrator) cleanupMembership(sid core.SessionID) {
	roomID, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room, ok := o.Rooms.GetRoom(roomID)
	if ok {
		room.RemoveMember(sid)
	}
	o.Registry.RemoveRoom(sid)
}

func (o *Orchestrator) EvictRoom(id domain.RoomID) {
	for _, snap := range o.Registry.MembersOfRoom(id) {
		o.KickBySID(snap.SID)
	}
	o.Rooms.StopRoom(id)
}
