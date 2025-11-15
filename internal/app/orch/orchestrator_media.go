package orch

import (
	"context"

	"github.com/dkeye/Voice/internal/core"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
)

func (o *Orchestrator) BindMediaHandlers(mc core.MediaConnection, sid core.SessionID) {
	mc.OnTrack(func(trackCtx context.Context, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		o.OnTrack(trackCtx, sid, track)
	})
	mc.OnClosed(func() { o.OnMediaDisconnect(sid) })
}

func (o *Orchestrator) OnMediaDisconnect(sid core.SessionID) {
	o.cleanupMedia(sid)
}

func (o *Orchestrator) cleanupMedia(sid core.SessionID) { // checked
	if o.Relays != nil {
		o.Relays.StopRelay(sid)

		RoomName, _, ok := o.Registry.RoomOf(sid)
		if ok {
			for _, snap := range o.Registry.MembersOfRoom(RoomName) {
				o.Relays.MarkSubscriberDelete(snap.SID, sid)
			}
		}
	}

	if sess, ok := o.Registry.GetSession(sid); ok {
		if mc := sess.Media(); mc != nil {
			mc.Close()
		}
	}
}

// OnTrack is called when a new remote media track appears for a given session.
func (o *Orchestrator) OnTrack(ctx context.Context, sid core.SessionID, track *webrtc.TrackRemote) { // checked
	if o.Relays == nil {
		return
	}
	if sess, ok := o.Registry.GetSession(sid); !ok || sess.Media() == nil {
		return
	}
	o.Relays.StartRelay(ctx, sid, track)

	roomName, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		log.Info().
			Str("module", "sfu").
			Str("sid", string(sid)).
			Msg("OnTrack: no room for sid")
		return
	}

	// Subscribe all existing members in the room to this speaker.
	for _, snap := range o.Registry.MembersOfRoom(roomName) {
		if snap.SID == sid {
			continue
		}
		pc := snap.Session.Media()
		if pc == nil {
			continue
		}
		o.Relays.Subscribe(sid, snap.SID, pc, track)
	}
}

// OnMediaReady is called when MediaConnection is attached to the session (offer/answer done).
// It subscribes this user as a subscriber to all existing relays in the same room.
func (o *Orchestrator) OnMediaReady(sid core.SessionID) { // checked
	if o.Relays == nil {
		return
	}
	roomName, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}

	// If there is no media connection yet, nothing to do.
	sess, ok := o.Registry.GetSession(sid)
	if !ok {
		return
	}
	mc := sess.Media()
	if mc == nil {
		return
	}

	for _, snap := range o.Registry.MembersOfRoom(roomName) {
		if snap.SID == sid {
			continue
		}
		srcTrack, ok := o.Relays.SrcTrack(snap.SID)
		if !ok {
			continue
		}
		o.Relays.Subscribe(snap.SID, sid, mc, srcTrack)
	}
}
