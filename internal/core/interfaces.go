package core

import "github.com/dkeye/Voice/internal/domain"

// Frame is a raw binary payload (e.g., audio frame).
type Frame []byte

// MemberConnection abstracts a transport endpoint (WS/WebRTC).
// Owned by the adapter; the adapter must Close() it.
type MemberConnection interface {
	ID() domain.UserID
	TrySend(Frame) error
	Close()
}

// MemberSession binds domain.Member and its transport endpoint.
// This is what a room stores and fans out to.
type MemberSession interface {
	Meta() *domain.Member
	Conn() MemberConnection
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
	AddMember(MemberSession)
	RemoveMember(id domain.UserID)
	MemberCount() int
	Broadcast(from domain.UserID, data Frame) PublishResult
	MembersSnapshot() []MemberDTO
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
