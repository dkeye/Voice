package core

import "github.com/dkeye/Voice/internal/domain"

type SessionID string

// MemberSession binds domain.Member and its transport endpoint.
// This is what a room stores and fans out to.
type MemberSession interface {
	Meta() *domain.Member
	Signal() SignalConnection
	Media() MediaConnection
	UpdateSignal(SignalConnection) MemberSession
	UpdateMedia(MediaConnection) MemberSession
}
