package app

import (
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
)

type Orchestrator struct {
	Registry *Registry
	Rooms    core.RoomFactory
	Policy   Policy
}

func (o *Orchestrator) Join(sid core.SessionID, roomName domain.RoomName, session core.MemberSession, cancel func()) {
	_, _, _, ok := o.Registry.RoomOf(sid)
	if ok {
		o.KickBySID(sid)
	}
	room := o.Rooms.GetOrCreate(roomName)
	room.AddMember(sid, session)
	o.Registry.BindSession(sid, session.Meta().User.ID, roomName, session, cancel)
}

func (o *Orchestrator) OnFrame(sid core.SessionID, data core.Frame) {
	roomName, _, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room := o.Rooms.GetOrCreate(roomName)

	res := room.Broadcast(sid, data)
	if o.Policy == nil {
		return
	}
	for _, slow := range res.Dropped {
		switch o.Policy.OnBackPressure(room, slow) {
		case KickMember:
			for _, snap := range o.Registry.MembersOfRoom(roomName) {
				if snap.Session == slow {
					o.KickBySID(snap.SID)
				}
			}
		case MarkSlow, DropFrame, NoAction:
		}
	}
}

func (o *Orchestrator) Move(sid core.SessionID, toRoomName string) bool {
	fromRoomName, _, session, ok := o.Registry.RoomOf(sid)
	if !ok {
		return false
	}
	to := domain.RoomName(toRoomName)
	if to == fromRoomName {
		return true
	}

	from := o.Rooms.GetOrCreate(fromRoomName)
	toRoom := o.Rooms.GetOrCreate(to)

	from.RemoveMember(sid)
	toRoom.AddMember(sid, session)
	return o.Registry.UpdateRoom(sid, to)
}

func (o *Orchestrator) KickBySID(sid core.SessionID) {
	roomName, _, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room := o.Rooms.GetOrCreate(roomName)
	room.RemoveMember(sid)
	o.Registry.Unbind(sid)
	o.Registry.Cancel(sid)
}

func (o *Orchestrator) OnDisconnect(sid core.SessionID) {
	roomName, _, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room := o.Rooms.GetOrCreate(roomName)
	room.RemoveMember(sid)
	o.Registry.Unbind(sid)
}

func (o *Orchestrator) EvictRoom(name domain.RoomName) {
	for _, snap := range o.Registry.MembersOfRoom(name) {
		o.KickBySID(snap.SID)
	}
	o.Rooms.StopRoom(name)
}
