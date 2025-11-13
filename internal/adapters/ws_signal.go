package adapters

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

type SignalWSController struct {
	Orch *app.Orchestrator
}

type wsSignalConn struct {
	conn *websocket.Conn
	send chan core.Frame
	once sync.Once
}

func (c *wsSignalConn) TrySend(f core.Frame) error {
	select {
	case c.send <- f:
		return nil
	default:
		return ErrBackpressure
	}
}

func (c *wsSignalConn) Close() {
	c.once.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (ctl *SignalWSController) HandleSignal(ctx context.Context, c *gin.Context) {
	sid := core.SessionID(c.GetString("client_token"))
	log.Printf("[signal] new WS connection sid=%s", sid)

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}

	conn := &wsSignalConn{
		conn: ws,
		send: make(chan core.Frame, 32),
	}

	user := ctl.Orch.Registry.GetOrCreateUser(sid)
	meta := domain.NewMember(user)
	sess := core.NewMemberSession(meta).UpdateSignal(conn)
	ctx, cancel := context.WithCancel(ctx)
	ctl.Orch.Registry.BindSignal(sid, sess, cancel)

	go ctl.writePump(ctx, conn)
	go ctl.readPump(ctx, sid, conn)
}

func (ctl *SignalWSController) writePump(ctx context.Context, c *wsSignalConn) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("[signal] writePump ctx done")
			return
		case data, ok := <-c.send:
			if !ok {
				log.Printf("[signal] writePump channel closed")
				return
			}
			if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				log.Printf("[signal] writePump set deadline error: %v", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("[signal] writePump write error: %v", err)
				return
			}
		}
	}
}

func (ctl *SignalWSController) readPump(ctx context.Context, sid core.SessionID, c *wsSignalConn) {
	defer func() {
		log.Printf("[signal] readPump closing sid=%s", sid)
		ctl.Orch.OnDisconnect(sid)
		c.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[signal] readPump ctx done sid=%s", sid)
			return
		default:
			_, data, err := c.conn.ReadMessage()
			if err != nil {
				log.Printf("[signal] readPump read error sid=%s: %v", sid, err)
				return
			}
			ctl.handleSignal(sid, c, data)
		}
	}
}

func (ctl *SignalWSController) handleSignal(sid core.SessionID, c *wsSignalConn, data []byte) {
	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		log.Println("bad json:", err)
		return
	}

	switch env.Type {
	case "join":
		ctl.handleJoin(sid, c, data)
	case "leave":
		ctl.handleLeave(sid, c)
	case "ping":
		ctl.handlePing(c)
	case "rename":
		ctl.handleRename(sid, c, data)
	case "whoami":
		ctl.handleWhoAmI(sid, c)
	case "offer":
		ctl.handleOffer(sid, c, data)
	case "candidate":
		ctl.handleCandidate(sid, c, data)
	default:
		log.Println("unknown signal:", env.Type)
	}
}

func (ctl *SignalWSController) sendJSON(c *wsSignalConn, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Println("sendJSON marshal:", err)
		return
	}
	_ = c.TrySend(b)
}

func (ctl *SignalWSController) handleJoin(
	sid core.SessionID,
	conn *wsSignalConn,
	data []byte,
) {
	type joinPayload struct {
		Type string `json:"type"`
		Room string `json:"room"`
		Name string `json:"name,omitempty"`
	}
	var p joinPayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Println("[signal] bad join payload:", err)
		return
	}

	roomName := domain.RoomName(p.Room)
	if roomName == "" {
		roomName = "main"
	}

	if p.Name != "" {
		ctl.Orch.Registry.UpdateUsername(sid, p.Name)
		log.Printf("[signal] rename on join sid=%s name=%s", sid, p.Name)
	}

	log.Printf("[signal] join sid=%s room=%s", sid, roomName)
	ctl.Orch.Join(sid, roomName)

	// Отдаём снапшот комнаты, чтобы клиент мог обновить UI.
	room := ctl.Orch.Rooms.GetOrCreate(roomName)
	resp := struct {
		Type    string           `json:"type"`
		Room    domain.RoomName  `json:"room"`
		Members []core.MemberDTO `json:"members"`
		Count   int              `json:"count"`
	}{
		Type:    "room_state",
		Room:    room.Room().Name,
		Members: room.MembersSnapshot(),
		Count:   room.MemberCount(),
	}
	ctl.sendJSON(conn, resp)
}

// handleLeave — выход из текущей комнаты, соединение при этом не рвётся.
func (ctl *SignalWSController) handleLeave(
	sid core.SessionID,
	conn *wsSignalConn,
) {
	log.Printf("[signal] leave sid=%s", sid)
	ctl.Orch.KickBySID(sid)
	ctl.sendJSON(conn, map[string]any{
		"type": "left",
	})
}

func (ctl *SignalWSController) handlePing(
	conn *wsSignalConn,
) {
	resp := struct {
		Type string `json:"type"`
	}{
		Type: "pong",
	}
	ctl.sendJSON(conn, resp)
}

func (ctl *SignalWSController) handleRename(
	sid core.SessionID,
	conn *wsSignalConn,
	data []byte,
) {
	type renamePayload struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	var p renamePayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Println("[signal] bad rename payload:", err)
		return
	}
	if p.Name == "" {
		ctl.sendJSON(conn, map[string]any{
			"type":  "error",
			"error": "empty name",
		})
		return
	}

	log.Printf("[signal] rename sid=%s name=%s", sid, p.Name)
	ctl.Orch.Registry.UpdateUsername(sid, p.Name)
	ctl.handleWhoAmI(sid, conn)
}

func (ctl *SignalWSController) handleWhoAmI(
	sid core.SessionID,
	conn *wsSignalConn,
) {
	user := ctl.Orch.Registry.GetOrCreateUser(sid)
	roomName, _, ok := ctl.Orch.Registry.RoomOf(sid)

	resp := struct {
		Type     string          `json:"type"`
		Username string          `json:"username"`
		Room     domain.RoomName `json:"room,omitempty"`
	}{
		Type:     "whoami",
		Username: user.Username,
	}
	if ok {
		resp.Room = roomName
	}
	ctl.sendJSON(conn, resp)
}

func (ctl *SignalWSController) sendCandidate(c *wsSignalConn, ci webrtc.ICECandidateInit) {
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
	conn *wsSignalConn,
	data []byte,
) {
	type offerPayload struct {
		Type string `json:"type"`
		SDP  string `json:"sdp"`
	}
	var p offerPayload
	if err := json.Unmarshal(data, &p); err != nil {
		log.Println("bad offer payload:", err)
		return
	}

	cfg := defaultWebRTCConfig()
	wc, err := NewWebRTCConnection(cfg, sid)
	if err != nil {
		log.Println("webrtc new pc:", err)
		return
	}

	wc.OnICECandidate(func(ci webrtc.ICECandidateInit) {
		ctl.sendCandidate(conn, ci)
	})

	if err = wc.Start(context.Background()); err != nil {
		log.Println("webrtc start:", err)
		wc.Close()
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  p.SDP,
	}

	answer, err := wc.ApplyOfferAndCreateAnswer(offer)
	if err != nil {
		log.Println("webrtc apply offer:", err)
		wc.Close()
		return
	}

	if sess, ok := ctl.Orch.Registry.GetSession(sid); ok {
		sess.UpdateMedia(wc)
	}

	ctl.sendJSON(conn, map[string]string{
		"type": "answer",
		"sdp":  answer.SDP,
	})
}

func (ctl *SignalWSController) handleCandidate(
	sid core.SessionID,
	_ *wsSignalConn,
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
		log.Println("bad candidate payload:", err)
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
		log.Println("candidate: no session for", sid)
		return
	}
	mc := sess.Media()
	if mc == nil {
		log.Println("candidate: no media connection for", sid)
		return
	}
	if err := mc.AddICECandidate(cand); err != nil {
		log.Println("add ice candidate:", err)
	}
}
