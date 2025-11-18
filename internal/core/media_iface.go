package core

import (
	"context"

	"github.com/pion/webrtc/v4"
)

type MediaConnection interface {
	// Start configures internal callbacks and binds the connection lifetime to ctx.
	Start(ctx context.Context) error
	// Close should stop all underlying media resources.
	Close()
	IsClosed() bool
	// AddICECandidate applies a remote ICE candidate.
	AddICECandidate(webrtc.ICECandidateInit) error
	// LocalDescription returns the current local SDP.
	ApplyAnswer(webrtc.SessionDescription) error
	CreateAndSetOffer() (*webrtc.SessionDescription, error)
	// OnICECandidate sets a callback for newly gathered local ICE candidates.
	OnICECandidate(func(webrtc.ICECandidateInit))
	// OnTrack sets a callback that will be invoked when a new remote track arrives.
	OnTrack(func(ctx context.Context, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver))
	// AddLocalTrack attaches a local static RTP track to the underlying PeerConnection.
	AddLocalTrack(track *webrtc.TrackLocalStaticRTP) (*webrtc.RTPSender, error)
	// OnClosed sets a callback for cleanup media session.
	OnClosed(func())
}
