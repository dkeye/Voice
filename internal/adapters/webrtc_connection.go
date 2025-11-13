package adapters

import (
	"context"
	"log"

	"github.com/dkeye/Voice/internal/core"
	"github.com/pion/webrtc/v4"
)

type WebRTCConnection struct {
	pc     *webrtc.PeerConnection
	sid    core.SessionID
	onICE  func(webrtc.ICECandidateInit)
	cancel context.CancelFunc
}

func defaultWebRTCConfig() webrtc.Configuration {
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
		log.Printf("[webrtc][%s] ICE state: %s", c.sid, s.String())
		if s == webrtc.ICEConnectionStateDisconnected ||
			s == webrtc.ICEConnectionStateFailed {
			cancel()
		}
	})

	c.pc.OnICECandidate(func(cand *webrtc.ICECandidate) {
		if cand != nil && c.onICE != nil {
			c.onICE(cand.ToJSON())
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
			log.Printf("[webrtc][%s] close error: %v", c.sid, err)
		} else {
			log.Printf("[webrtc][%s] closed", c.sid)
		}
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
