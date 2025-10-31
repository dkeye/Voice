package app

import (
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
)

// Orchestrator bridges transport events to core rooms and applies policies.
type Orchestrator struct {
	Rooms  core.RoomFactory
	Policy Policy
}

func (o *Orchestrator) OnFrameReceived(room core.RoomService, from domain.UserID, data core.Frame) {
	res := room.Broadcast(from, data)
	for _, slow := range res.Dropped {
		switch o.Policy.OnBackPressure(room, slow) {
		case KickMember:
			id := slow.Meta().User.ID
			room.RemoveMember(id)
			slow.Conn().Close()
		case MarkSlow, DropFrame, NoAction:
			// TODO: metrics/logs if needed
		}
	}
}
