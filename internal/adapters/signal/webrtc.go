package signal

import (
	"context"
	"encoding/json"

	"github.com/dkeye/Voice/internal/adapters/rtc"
	"github.com/dkeye/Voice/internal/core"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
)

func (ctl *SignalWSController) sendCandidate(c *WsSignalConn, ci webrtc.ICECandidateInit) {
	resp := struct {
		Type          string `json:"type"`
		Candidate     string `json:"candidate"`
		SDPMid        string `json:"sdpMid,omitempty"`
		SDPMLineIndex uint16 `json:"sdpMLineIndex,omitempty"`
	}{
		Type:      "candidate",
		Candidate: ci.Candidate,
	}
	if ci.SDPMid != nil {
		resp.SDPMid = *ci.SDPMid
	}
	if ci.SDPMLineIndex != nil {
		resp.SDPMLineIndex = *ci.SDPMLineIndex
	}
	ctl.sendJSON(c, resp)
}

func (ctl *SignalWSController) handleOffer(
	sid core.SessionID,
	conn *WsSignalConn,
	data []byte,
) {
	type offerPayload struct {
		Type string `json:"type"`
		SDP  string `json:"sdp"`
	}
	var p offerPayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("bad offer payload")
		return
	}

	cfg := rtc.DefaultWebRTCConfig()
	wc, err := rtc.NewWebRTCConnection(cfg, sid)
	if err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("webrtc new pc")
		return
	}
	wc.OnNegotiationNeeded(func() {
		ctl.handleNegotiationNeeded(sid, conn, wc)
	})

	wc.OnICECandidate(func(ci webrtc.ICECandidateInit) {
		ctl.sendCandidate(conn, ci)
	})

	ctl.Orch.BindMediaHandlers(wc, sid)

	if err = wc.Start(context.Background()); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("webrtc start")
		wc.Close()
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  p.SDP,
	}

	if err = wc.ApplyOffer(offer); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("webrtc apply offer")
		wc.Close()
		return
	}

	answer, err := wc.CreateAndSetAnswer()
	if err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("webrtc create/set answer")
		wc.Close()
		return
	}

	if sess, ok := ctl.Orch.Registry.GetSession(sid); ok {
		sess.UpdateMedia(wc)
		ctl.Orch.OnMediaReady(sid)
	}

	ctl.sendJSON(conn, map[string]string{
		"type": "answer",
		"sdp":  answer.SDP,
	})
}

func (ctl *SignalWSController) handleAnswer(
	sid core.SessionID,
	_ *WsSignalConn,
	data []byte,
) {
	type answerPayload struct {
		Type string `json:"type"`
		SDP  string `json:"sdp"`
	}
	var p answerPayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Error().Err(err).Msg("bad answer payload")
		return
	}

	sess, ok := ctl.Orch.Registry.GetSession(sid)
	if !ok {
		return
	}

	mc := sess.Media()
	if mc == nil {
		return
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  p.SDP,
	}

	if err := mc.ApplyAnswer(answer); err != nil {
		log.Error().Err(err).Msg("set remote answer")
		return
	}
}

func (ctl *SignalWSController) handleCandidate(
	sid core.SessionID,
	_ *WsSignalConn,
	data []byte,
) {
	type candidatePayload struct {
		Type          string `json:"type"`
		Candidate     string `json:"candidate"`
		SDPMid        string `json:"sdpMid"`
		SDPMLineIndex uint16 `json:"sdpMLineIndex"`
	}
	var p candidatePayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("bad candidate payload")
		return
	}
	if p.Candidate == "" {
		log.Debug().Str("module", "signal").Str("sid", string(sid)).Msg("end-of-candidates")
		return
	}

	cand := webrtc.ICECandidateInit{
		Candidate: p.Candidate,
	}
	if p.SDPMid != "" {
		cand.SDPMid = &p.SDPMid
	}
	cand.SDPMLineIndex = &p.SDPMLineIndex

	sess, ok := ctl.Orch.Registry.GetSession(sid)
	if !ok {
		log.Warn().Str("module", "signal").Str("sid", string(sid)).Msg("candidate: no session for")
		return
	}
	mc := sess.Media()
	if mc == nil {
		log.Warn().Str("module", "signal").Str("sid", string(sid)).Msg("candidate: no media connection for")
		return
	}
	if err := mc.AddICECandidate(cand); err != nil {
		log.Error().Err(err).Str("module", "signal").Msg("add ice candidate")
	}
}

func (ctl *SignalWSController) handleNegotiationNeeded(
	sid core.SessionID,
	conn *WsSignalConn,
	mc core.MediaConnection,
) {
	offer, err := mc.CreateAndSetOffer()
	if err != nil {
		log.Error().Err(err).Msg("negotiation offer failed")
		return
	}

	ctl.sendJSON(conn, map[string]string{
		"type": "offer",
		"sdp":  offer.SDP,
	})
}
