package app

import "github.com/dkeye/Voice/internal/core"

type BackpressureAction int

const (
	NoAction BackpressureAction = iota
	MarkSlow
	KickMember
	DropFrame
)

type Policy interface {
	OnBackPressure(room core.RoomService, member core.MemberSession) BackpressureAction
}

type SimplePolicy struct{}

func (SimplePolicy) OnBackPressure(room core.RoomService, member core.MemberSession) BackpressureAction {
	return KickMember
}
