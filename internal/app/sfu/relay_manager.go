package sfu

import (
	"context"
	"sync"

	"github.com/dkeye/Voice/internal/core"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
)

type RelayManager struct {
	mu     sync.RWMutex
	relays map[core.SessionID]*Relay
}

func NewRelayManager() *RelayManager {
	return &RelayManager{
		relays: make(map[core.SessionID]*Relay),
	}
}

// StartRelay creates a new Relay for the given speaker SID and starts its loop.
func (m *RelayManager) StartRelay(ctx context.Context, sid core.SessionID, track *webrtc.TrackRemote) {
	logger := log.With().
		Str("module", "relay").
		Str("sid", string(sid)).
		Logger()

	relayCtx, cancel := context.WithCancel(ctx)
	relay := NewRelay(track, cancel)

	m.mu.Lock()
	if old, ok := m.relays[sid]; ok {
		logger.Info().Msg("replacing existing relay for sid")
		old.markAllDelete()
		if old.cancel != nil {
			old.cancel()
		}
	}
	m.relays[sid] = relay
	m.mu.Unlock()

	logger.Info().Msg("starting relay loop")

	go relay.loop(relayCtx, sid, &logger)
}

// AddSubscriber attaches an OutTrack to the relay of srcSID for dstSID.
func (m *RelayManager) AddSubscriber(srcSID, dstSID core.SessionID, localTrack *webrtc.TrackLocalStaticRTP) {
	m.mu.RLock()
	relay, ok := m.relays[srcSID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	ot := NewOutTrack(localTrack)
	relay.AddOutTrack(dstSID, ot)
}

// MarkSubscriberDelete marks subscriber's OutTrack as TrackStateDelete.
func (m *RelayManager) MarkSubscriberDelete(srcSID, dstSID core.SessionID) {
	m.mu.RLock()
	relay, ok := m.relays[srcSID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	relay.mu.RLock()
	ot, ok := relay.outTracks[dstSID]
	relay.mu.RUnlock()
	if !ok {
		return
	}
	ot.MarkDelete()
}

// StopRelay stops a relay and removes it from the manager.
func (m *RelayManager) StopRelay(srcSID core.SessionID) {
	m.mu.Lock()
	relay, ok := m.relays[srcSID]
	if ok {
		delete(m.relays, srcSID)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	relay.markAllDelete()
}

// HasRelay reports whether a relay exists for sid.
func (m *RelayManager) HasRelay(sid core.SessionID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.relays[sid]
	return ok
}

// SrcTrack returns the source track for a given relay.
func (m *RelayManager) SrcTrack(sid core.SessionID) (*webrtc.TrackRemote, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	relay, ok := m.relays[sid]
	if !ok {
		return nil, false
	}
	return relay.Src, true
}
