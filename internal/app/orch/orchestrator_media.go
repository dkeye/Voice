package orch

import (
	"context"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
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
	o.cleanupMembership(sid)
	o.Registry.Unbind(sid)
}

func (o *Orchestrator) cleanupMedia(sid core.SessionID) {
	if sess, ok := o.Registry.GetSession(sid); ok {
		if mc := sess.Media(); mc != nil {
			mc.Close()
		}
	}
	if o.Relays != nil {
		o.Relays.StopRelay(sid)
	}
}

// OnTrack is called when a new remote media track appears for a given session.
func (o *Orchestrator) OnTrack(ctx context.Context, sid core.SessionID, track *webrtc.TrackRemote) {
	if o.Relays == nil {
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
		o.subscribeSpeakerToSubscriber(roomName, sid, snap.SID, track)
	}
}

// OnMediaReady is called when MediaConnection is attached to the session (offer/answer done).
// It subscribes this user as a subscriber to all existing relays in the same room.
func (o *Orchestrator) OnMediaReady(sid core.SessionID) {
	if o.Relays == nil {
		return
	}
	roomName, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}

	// If there is no media connection yet, nothing to do.
	sess, ok := o.Registry.GetSession(sid)
	if !ok || sess.Media() == nil {
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
		o.subscribeSpeakerToSubscriber(roomName, snap.SID, sid, srcTrack)
	}
}

// subscribeSpeakerToSubscriber creates a local track for dstSID and attaches it to dstSID's PeerConnection,
// then registers it in the corresponding relay.
func (o *Orchestrator) subscribeSpeakerToSubscriber(
	roomName domain.RoomName,
	srcSID, dstSID core.SessionID,
	srcTrack *webrtc.TrackRemote,
) {
	if o.Relays == nil {
		return
	}

	dstSess, ok := o.Registry.GetSession(dstSID)
	if !ok {
		return
	}
	mc := dstSess.Media()
	if mc == nil {
		return
	}

	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		srcTrack.Codec().RTPCodecCapability,
		srcTrack.ID(),
		srcTrack.StreamID(),
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("module", "sfu").
			Str("src_sid", string(srcSID)).
			Str("dst_sid", string(dstSID)).
			Msg("create local track")
		return
	}

	if _, err := mc.AddLocalTrack(localTrack); err != nil {
		log.Error().
			Err(err).
			Str("module", "sfu").
			Str("src_sid", string(srcSID)).
			Str("dst_sid", string(dstSID)).
			Msg("add local track to peerconnection")
		return
	}

	o.Relays.AddSubscriber(srcSID, dstSID, localTrack)
	log.Info().
		Str("module", "sfu").
		Str("room", string(roomName)).
		Str("src_sid", string(srcSID)).
		Str("dst_sid", string(dstSID)).
		Msg("subscriber added to relay")
}
