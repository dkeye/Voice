package orch

import (
	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/app/sfu"
	"github.com/dkeye/Voice/internal/core"
)

type Orchestrator struct {
	Registry *app.Registry
	Rooms    core.RoomManager
	Policy   app.Policy
	Relays   *sfu.RelayManager
}

func NewOrchestrator(
	registry *app.Registry,
	roomsManager core.RoomManager,
	policy app.Policy,
	relayManager *sfu.RelayManager,
) *Orchestrator {
	o := &Orchestrator{
		Registry: registry,
		Rooms:    roomsManager,
		Policy:   policy,
		Relays:   relayManager,
	}
	return o
}

func (o *Orchestrator) OnFrame(sid core.SessionID, data core.Frame) {
	roomID, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room, ok := o.Rooms.GetRoom(roomID)
	if !ok {
		return
	}

	res := room.Broadcast(sid, data)
	if o.Policy == nil {
		return
	}
	for _, slow := range res.Dropped {
		switch o.Policy.OnBackPressure(room, slow) {
		case app.KickMember:
			for _, snap := range o.Registry.MembersOfRoom(roomID) {
				if snap.Session == slow {
					o.KickBySID(snap.SID)
				}
			}
		case app.MarkSlow, app.DropFrame, app.NoAction:
		}
	}
}
