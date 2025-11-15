package sfu

import (
	"sync/atomic"

	"github.com/pion/webrtc/v4"
)

type TrackState int32

const (
	TrackStateOk TrackState = iota
	TrackStateMuted
	TrackStateDelete
)

// OutTrack represents a single outgoing track to a subscriber.
type OutTrack struct {
	Track *webrtc.TrackLocalStaticRTP
	state atomic.Int32 // Zero by default (TrackStateOk)
}

func NewOutTrack(track *webrtc.TrackLocalStaticRTP) *OutTrack {
	return &OutTrack{Track: track}
}

func (ot *OutTrack) GetState() TrackState {
	return TrackState(ot.state.Load())
}

func (ot *OutTrack) MarkOk() {
	ot.state.Store(int32(TrackStateOk))
}

func (ot *OutTrack) MarkMuted() {
	ot.state.Store(int32(TrackStateMuted))
}

func (ot *OutTrack) MarkDelete() {
	ot.state.Store(int32(TrackStateDelete))
}
