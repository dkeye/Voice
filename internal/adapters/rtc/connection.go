package rtc

import (
	"context"

	"github.com/dkeye/Voice/internal/core"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
)

type WebRTCConnection struct {
	pc     *webrtc.PeerConnection
	sid    core.SessionID
	onICE  func(webrtc.ICECandidateInit)
	cancel context.CancelFunc

	onTrack  func(ctx context.Context, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)
	onClosed (func())
}

func DefaultWebRTCConfig() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
}

func NewWebRTCConnection(cfg webrtc.Configuration, sid core.SessionID) (*WebRTCConnection, error) {
	pc, err := webrtc.NewPeerConnection(cfg)
	if err != nil {
		return nil, err
	}
	return &WebRTCConnection{pc: pc, sid: sid}, nil
}

func (c *WebRTCConnection) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.pc.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		log.Info().Str("module", "webrtc").Str("sid", string(c.sid)).Str("ice_state", s.String()).Msg("ICE state")
		if s == webrtc.ICEConnectionStateDisconnected ||
			s == webrtc.ICEConnectionStateFailed ||
			s == webrtc.ICEConnectionStateClosed {
			cancel()
		}
	})

	c.pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Info().Str("module", "webrtc").Str("sid", string(c.sid)).Str("peer_connection_state", s.String()).Msg("Peer state")
		if s == webrtc.PeerConnectionStateFailed ||
			s == webrtc.PeerConnectionStateClosed {
			if c.onClosed != nil {
				c.onClosed()
			}
		}
	})

	c.pc.OnICECandidate(func(cand *webrtc.ICECandidate) {
		if cand != nil && c.onICE != nil {
			c.onICE(cand.ToJSON())
		}
	})

	c.pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Info().
			Str("module", "webrtc").
			Str("sid", string(c.sid)).
			Str("kind", track.Kind().String()).
			Str("track_id", track.ID()).
			Str("stream_id", track.StreamID()).
			Msg("OnTrack received")
		if c.onTrack != nil {
			c.onTrack(ctx, track, receiver)
		}
	})

	return nil
}

func (c *WebRTCConnection) ApplyOfferAndCreateAnswer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if err := c.pc.SetRemoteDescription(offer); err != nil {
		return nil, err
	}
	answer, err := c.pc.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	gatherComplete := webrtc.GatheringCompletePromise(c.pc)
	if err := c.pc.SetLocalDescription(answer); err != nil {
		return nil, err
	}
	<-gatherComplete

	return c.pc.LocalDescription(), nil
}

func (c *WebRTCConnection) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.pc != nil {
		if err := c.pc.Close(); err != nil {
			log.Error().Err(err).Str("module", "webrtc").Str("sid", string(c.sid)).Msg("close error")
		} else {
			log.Info().Str("module", "webrtc").Str("sid", string(c.sid)).Msg("closed")
		}
	}
	if c.onClosed != nil {
		c.onClosed()
	}
}

func (c *WebRTCConnection) AddICECandidate(ci webrtc.ICECandidateInit) error {
	return c.pc.AddICECandidate(ci)
}

func (c *WebRTCConnection) LocalDescription() *webrtc.SessionDescription {
	return c.pc.LocalDescription()
}

func (c *WebRTCConnection) OnICECandidate(fn func(webrtc.ICECandidateInit)) {
	c.onICE = fn
}

// OnTrack sets application-level callback for remote tracks.
func (c *WebRTCConnection) OnTrack(fn func(ctx context.Context, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)) {
	c.onTrack = fn
}

// OnClosed sets application-level callback for cleanup tracks
func (c *WebRTCConnection) OnClosed(fn func()) { c.onClosed = fn }

// AddLocalTrack attaches a local static RTP track to the PeerConnection.
func (c *WebRTCConnection) AddLocalTrack(track *webrtc.TrackLocalStaticRTP) (*webrtc.RTPSender, error) {
	sender, err := c.pc.AddTrack(track)
	if err != nil {
		return nil, err
	}
	return sender, nil
}
