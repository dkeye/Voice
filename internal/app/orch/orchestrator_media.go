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

func (o *Orchestrator) cleanupMedia(sid core.SessionID) {
	if o.Relays != nil {
		o.Relays.StopRelay(sid)

		for _, snap := range o.Registry.RoomMates(sid) {
			o.Relays.MarkSubscriberDelete(snap.SID, sid)
		}
	}

	if sess, ok := o.Registry.GetSession(sid); ok {
		if mc := sess.Media(); mc != nil {
			mc.Close()
		}
	}
}

// OnTrack is called when a new remote media track appears for a given session.
func (o *Orchestrator) OnTrack(ctx context.Context, sid core.SessionID, track *webrtc.TrackRemote) {
	if o.Relays == nil {
		return
	}
	if sess, ok := o.Registry.GetSession(sid); !ok || sess.Media() == nil {
		return
	}
	o.Relays.StartRelay(ctx, sid, track)

	roomID, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		log.Info().
			Str("module", "sfu").
			Str("sid", string(sid)).
			Msg("OnTrack: no room for sid")
		return
	}

	// Subscribe all existing members in the room to this speaker.
	for _, snap := range o.Registry.MembersOfRoom(roomID) {
		if snap.SID == sid {
			continue
		}
		mc := snap.Session.Media()
		if mc == nil {
			continue
		}
		if err := o.Relays.Subscribe(sid, snap.SID, mc, track); err != nil {
			log.Error().
				Err(err).
				Str("module", "sfu").
				Str("src_sid", string(sid)).
				Str("dst_sid", string(snap.SID)).
				Msg("Subscribe in OnTrack failed")
			continue
		}
	}
}

// OnMediaReady is called when MediaConnection is attached to the session (offer/answer done).
// It subscribes this user as a subscriber to all existing relays in the same room.
func (o *Orchestrator) OnMediaReady(sid core.SessionID) {
	if o.Relays == nil {
		return
	}
	roomID, _, ok := o.Registry.RoomOf(sid)
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

	for _, snap := range o.Registry.MembersOfRoom(roomID) {
		if snap.SID == sid {
			continue
		}
		track, ok := o.Relays.SrcTrack(snap.SID)
		if !ok {
			continue
		}
		if err := o.Relays.Subscribe(snap.SID, sid, mc, track); err != nil {
			log.Error().
				Err(err).
				Str("module", "sfu").
				Str("src_sid", string(sid)).
				Str("dst_sid", string(snap.SID)).
				Msg("Subscribe in OnMediaReady failed")
			continue
		}

	}
}
