package sfu

import (
	"context"
	"maps"
	"sync"

	"github.com/dkeye/Voice/internal/core"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
)

type Relay struct {
	Src *webrtc.TrackRemote

	mu        sync.RWMutex
	outTracks map[core.SessionID]*OutTrack

	cancel context.CancelFunc
}

func NewRelay(src *webrtc.TrackRemote, cancel context.CancelFunc) *Relay {
	return &Relay{
		Src:       src,
		outTracks: make(map[core.SessionID]*OutTrack),
		cancel:    cancel,
	}
}

// loop reads RTP packets from the source track and forwards them to all OutTracks.
func (r *Relay) loop(ctx context.Context, _ core.SessionID, logger *zerolog.Logger) { // checked
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

func (r *Relay) forward(pkt *rtp.Packet, logger *zerolog.Logger) { // checked
	snapshot := make(map[core.SessionID]*OutTrack, len(r.outTracks))
	r.mu.RLock()
	maps.Copy(snapshot, r.outTracks)
	r.mu.RUnlock()

	dirty := make([]core.SessionID, 0, len(snapshot))
	for dstSID, ot := range snapshot {
		switch ot.GetState() {
		case TrackStateDelete:
			dirty = append(dirty, dstSID)
		case TrackStateMuted:
		case TrackStateOk:
			if err := ot.Track.WriteRTP(pkt); err != nil {
				logger.Error().
					Err(err).
					Str("dst_sid", string(dstSID)).
					Msg("relay write RTP error, marking outtrack as delete")
				ot.MarkDelete()
				dirty = append(dirty, dstSID)
			}
		}
	}

	// Cleanup is done outside the RLock.
	if len(dirty) > 0 {
		r.cleanupDeleted(dirty)
	}
}

func (r *Relay) cleanupDeleted(dirty []core.SessionID) { // checked
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sid := range dirty {
		delete(r.outTracks, sid)
	}
}

func (r *Relay) markAllDelete() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ot := range r.outTracks {
		ot.MarkDelete()
	}
}

func (r *Relay) AddOutTrack(dst core.SessionID, ot *OutTrack) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outTracks[dst] = ot
}
