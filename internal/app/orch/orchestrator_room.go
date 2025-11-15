package orch

import (
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

func (o *Orchestrator) Join(sid core.SessionID, roomName domain.RoomName) {
	RoomName, _, ok := o.Registry.RoomOf(sid)
	if ok {
		o.KickBySID(sid)
		log.Info().Str("sid", string(sid)).Str("from_room", string(RoomName)).Msg("kicked from room")
	}
	if session, ok := o.Registry.GetSession(sid); ok {
		room := o.Rooms.GetOrCreate(roomName)
		room.AddMember(sid, session)
		o.Registry.UpdateRoom(sid, roomName)
		log.Info().Str("sid", string(sid)).Str("room", string(roomName)).Msg("added to room")
	}
}

func (o *Orchestrator) Move(sid core.SessionID, toRoomName string) bool {
	fromRoomName, session, ok := o.Registry.RoomOf(sid)
	if !ok {
		return false
	}
	to := domain.RoomName(toRoomName)
	if to == fromRoomName {
		return true
	}

	// Unsubscribe from speakers in the old room, if any.
	if o.Relays != nil {
		for _, snap := range o.Registry.MembersOfRoom(fromRoomName) {
			o.Relays.MarkSubscriberDelete(snap.SID, sid)
		}
	}

	from := o.Rooms.GetOrCreate(fromRoomName)
	toRoom := o.Rooms.GetOrCreate(to)

	from.RemoveMember(sid)
	toRoom.AddMember(sid, session)
	ok = o.Registry.UpdateRoom(sid, to)

	if ok {
		o.OnMediaReady(sid)
	}

	return ok
}

func (o *Orchestrator) KickBySID(sid core.SessionID) {
	o.cleanupMedia(sid)
	o.cleanupMembership(sid)
}

func (o *Orchestrator) cleanupMembership(sid core.SessionID) {
	roomName, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room := o.Rooms.GetOrCreate(roomName)
	room.RemoveMember(sid)
	o.Registry.RemoveRoom(sid)
}

func (o *Orchestrator) EvictRoom(name domain.RoomName) {
	for _, snap := range o.Registry.MembersOfRoom(name) {
		o.KickBySID(snap.SID)
	}
	o.Rooms.StopRoom(name)
}
