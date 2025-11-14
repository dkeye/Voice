package core

import (
	"context"

	"github.com/dkeye/Voice/internal/domain"
	"github.com/pion/webrtc/v4"
)

// Frame is a raw binary payload.
type Frame []byte

type SessionID string

// SignalConnection abstracts for a system messaging transport
// Owned by the adapter; the adapter must Close() it.
type SignalConnection interface {
	TrySend(Frame) error
	Close()
}

type MediaConnection interface {
	// Start configures internal callbacks and binds the connection lifetime to ctx.
	Start(ctx context.Context) error
	// Close should stop all underlying media resources.
	Close()
	// AddICECandidate applies a remote ICE candidate.
	AddICECandidate(webrtc.ICECandidateInit) error
	// LocalDescription returns the current local SDP.
	LocalDescription() *webrtc.SessionDescription
	// OnICECandidate sets a callback for newly gathered local ICE candidates.
	OnICECandidate(func(webrtc.ICECandidateInit))
	// OnTrack sets a callback that will be invoked when a new remote track arrives.
	OnTrack(func(ctx context.Context, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver))
	// AddLocalTrack attaches a local static RTP track to the underlying PeerConnection.
	AddLocalTrack(track *webrtc.TrackLocalStaticRTP) (*webrtc.RTPSender, error)
	// OnClosed sets a callback for cleanup media session.
	OnClosed(func())
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
