package core

import (
	"context"

	"github.com/dkeye/Voice/internal/domain"
	"github.com/pion/webrtc/v4"
)

// Frame is a raw binary payload (e.g., audio frame).
type Frame []byte

type SessionID string

// SignalConnection abstracts for a system messaging transport
// Owned by the adapter; the adapter must Close() it.
type SignalConnection interface {
	TrySend(Frame) error
	Close()
}

type MediaConnection interface {
	Start(ctx context.Context) error
	Close()
	AddICECandidate(webrtc.ICECandidateInit) error
	LocalDescription() *webrtc.SessionDescription
	OnICECandidate(func(webrtc.ICECandidateInit))
}

// MemberSession binds domain.Member and its transport endpoint.
// This is what a room stores and fans out to.
type MemberSession interface {
	Meta() *domain.Member
	Signal() SignalConnection
	Media() MediaConnection
	UpdateSignal(SignalConnection) MemberSession
	UpdateMedia(MediaConnection) MemberSession
}

// PublishResult reports delivery stats/backpressure to orchestrator.
type PublishResult struct {
	SendTo  int
	Dropped []MemberSession
}

// MemberDTO is a read-only view for APIs (no transport fields).
type MemberDTO struct {
	ID       domain.UserID `json:"id"`
	Username string        `json:"username"`
}

// RoomService is the core-facing API of a room.
// It owns the membership set but never touches transport resources.
type RoomService interface {
	Room() *domain.Room
	MemberCount() int
	MembersSnapshot() []MemberDTO

	AddMember(sid SessionID, ms MemberSession)
	RemoveMember(sid SessionID)
	Broadcast(from SessionID, data Frame) PublishResult
}

type RoomInfo struct {
	Name        domain.RoomName `json:"name"`
	MemberCount int             `json:"client_count"`
}

type RoomFactory interface {
	GetOrCreate(name domain.RoomName) RoomService
	List() []RoomInfo
	StopRoom(name domain.RoomName)
}
