package app

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/dkeye/Voice/internal/core"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
)

const (
	TrackStateOk int32 = iota
	TrackStateMuted
	TrackStateDelete
)

// OutTrack represents a single outgoing track to a subscriber.
type OutTrack struct {
	Track *webrtc.TrackLocalStaticRTP
	State int32 // accessed atomically (TrackStateOk/Muted/Delete)
}

type Relay struct {
	Src *webrtc.TrackRemote

	mu        sync.RWMutex
	outTracks map[core.SessionID]*OutTrack

	cancel context.CancelFunc
}

// loop reads RTP packets from the source track and forwards them to all OutTracks.
func (r *Relay) loop(ctx context.Context, sid core.SessionID, logger *zerolog.Logger) {
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("relay ctx done, marking all out tracks for delete")
			r.markAllDelete()
			return
		default:
		}
		pkt, _, err := r.Src.ReadRTP()
		if err != nil {
			logger.Error().Err(err).Msg("relay read RTP error, stopping")
			r.markAllDelete()
			return
		}
		r.forward(pkt, logger)
	}
}

func (r *Relay) forward(pkt *rtp.Packet, logger *zerolog.Logger) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dirty := false
	for dstSID, ot := range r.outTracks {
		state := atomic.LoadInt32(&ot.State)
		if state == TrackStateDelete {
			dirty = true
			continue
		}
		if state == TrackStateMuted {
			continue
		}
		if err := ot.Track.WriteRTP(pkt); err != nil {
			logger.Error().
				Err(err).
				Str("dst_sid", string(dstSID)).
				Msg("relay write RTP error, marking outtrack as delete")
			atomic.StoreInt32(&ot.State, TrackStateDelete)
			dirty = true
		}
	}

	// Cleanup is done outside the RLock.
	if dirty {
		r.cleanupDeleted()
	}
}

func (r *Relay) cleanupDeleted() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for sid, ot := range r.outTracks {
		if atomic.LoadInt32(&ot.State) == TrackStateDelete {
			delete(r.outTracks, sid)
		}
	}
}

func (r *Relay) markAllDelete() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ot := range r.outTracks {
		atomic.StoreInt32(&ot.State, TrackStateDelete)
	}
}
