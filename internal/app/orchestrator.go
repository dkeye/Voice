package app

import (
	"context"

	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
)

type Orchestrator struct {
	Registry *Registry
	Rooms    core.RoomFactory
	Policy   Policy
	Relays   *RelayManager
}

func (o *Orchestrator) Join(sid core.SessionID, roomName domain.RoomName) {
	RoomName, _, ok := o.Registry.RoomOf(sid)
	if ok {
		o.KickBySID(sid)
		log.Info().Str("sid", string(sid)).Str("from_room", string(RoomName)).Msg("kicked from room")
	}
	if session, ok := o.Registry.GetSession(sid); ok {
		room := o.Rooms.GetOrCreate(roomName)
		room.AddMember(sid, session)
		o.Registry.UpdateRoom(sid, roomName)
		log.Info().Str("sid", string(sid)).Str("room", string(roomName)).Msg("added to room")
	}
}

func (o *Orchestrator) OnFrame(sid core.SessionID, data core.Frame) {
	roomName, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room := o.Rooms.GetOrCreate(roomName)

	res := room.Broadcast(sid, data)
	if o.Policy == nil {
		return
	}
	for _, slow := range res.Dropped {
		switch o.Policy.OnBackPressure(room, slow) {
		case KickMember:
			for _, snap := range o.Registry.MembersOfRoom(roomName) {
				if snap.Session == slow {
					o.KickBySID(snap.SID)
				}
			}
		case MarkSlow, DropFrame, NoAction:
		}
	}
}

func (o *Orchestrator) Move(sid core.SessionID, toRoomName string) bool {
	fromRoomName, session, ok := o.Registry.RoomOf(sid)
	if !ok {
		return false
	}
	to := domain.RoomName(toRoomName)
	if to == fromRoomName {
		return true
	}

	// Unsubscribe from speakers in the old room, if any.
	if o.Relays != nil {
		for _, snap := range o.Registry.MembersOfRoom(fromRoomName) {
			o.Relays.MarkSubscriberDelete(snap.SID, sid)
		}
	}

	from := o.Rooms.GetOrCreate(fromRoomName)
	toRoom := o.Rooms.GetOrCreate(to)

	from.RemoveMember(sid)
	toRoom.AddMember(sid, session)
	ok = o.Registry.UpdateRoom(sid, to)

	if ok {
		o.OnMediaReady(sid)
	}

	return ok
}

func (o *Orchestrator) KickBySID(sid core.SessionID) {
	if sess, ok := o.Registry.GetSession(sid); ok {
		if mc := sess.Media(); mc != nil {
			mc.Close()
		}
	}

	if o.Relays != nil {
		o.Relays.StopRelay(sid)
	}

	roomName, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room := o.Rooms.GetOrCreate(roomName)
	room.RemoveMember(sid)
	o.Registry.RemoveRoom(sid)
}

func (o *Orchestrator) OnMediaDisconnect(sid core.SessionID) {
	if sess, ok := o.Registry.GetSession(sid); ok {
		if mc := sess.Media(); mc != nil {
			mc.Close()
		}
	}

	if o.Relays != nil {
		o.Relays.StopRelay(sid)
	}

	roomName, _, ok := o.Registry.RoomOf(sid)
	if !ok {
		return
	}
	room := o.Rooms.GetOrCreate(roomName)
	room.RemoveMember(sid)
	o.Registry.Unbind(sid)
}

func (o *Orchestrator) OnSignalDisconnect(sid core.SessionID) {
	// Nothing todo, signaling must reconnect
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

func (o *Orchestrator) EvictRoom(name domain.RoomName) {
	for _, snap := range o.Registry.MembersOfRoom(name) {
		o.KickBySID(snap.SID)
	}
	o.Rooms.StopRoom(name)
}
