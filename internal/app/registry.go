package app

import (
	"context"
	"sync"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

type sessionEntry struct {
	RoomName domain.RoomName
	Session  core.MemberSession
	Cancel   context.CancelFunc
}

type Registry struct {
	mu       sync.RWMutex
	sessions map[core.SessionID]*sessionEntry
	users    map[core.SessionID]*domain.User
}

func NewRegistry() *Registry {
	return &Registry{
		sessions: make(map[core.SessionID]*sessionEntry),
		users:    make(map[core.SessionID]*domain.User),
	}
}

func (r *Registry) GetOrCreateUser(sid core.SessionID) *domain.User {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.users[sid]; ok {
		return u
	}
	u := &domain.User{ID: domain.UserID(sid), Username: "guest"}
	r.users[sid] = u
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Msg("created new user")
	return u
}

func (r *Registry) UpdateUsername(sid core.SessionID, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.users[sid]; ok {
		u.Username = name
		log.Info().Str("module", "app.registry").Str("sid", string(sid)).Str("username", name).Msg("updated username")
	}
}

func (r *Registry) BindSession(
	sid core.SessionID,
	roomName domain.RoomName,
	sess core.MemberSession,
	cancel context.CancelFunc,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sid] = &sessionEntry{
		RoomName: roomName,
		Session:  sess,
		Cancel:   cancel,
	}
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Str("room", string(roomName)).Msg("bound session")
}

func (r *Registry) BindSignal(sid core.SessionID, sess core.MemberSession, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sid] = &sessionEntry{Session: sess, Cancel: cancel}
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Msg("bound signal")
}

func (r *Registry) GetSession(sid core.SessionID) (core.MemberSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.sessions[sid]; ok {
		return e.Session, true
	}
	return nil, false
}

func (r *Registry) Unbind(sid core.SessionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sid)
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Msg("unbind session")
}

func (r *Registry) RoomOf(sid core.SessionID) (domain.RoomName, core.MemberSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.sessions[sid]
	if !ok || entry.RoomName == "" {
		return "", nil, false
	}
	return entry.RoomName, entry.Session, true
}

func (r *Registry) UpdateRoom(sid core.SessionID, newRoom domain.RoomName) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.sessions[sid]
	if !ok {
		return false
	}
	entry.RoomName = newRoom
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Str("room", string(newRoom)).Msg("updated room")
	return true
}

func (r *Registry) RemoveRoom(sid core.SessionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.sessions[sid]; ok {
		entry.RoomName = ""
	}
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Msg("removed room association")
}

type regSnap struct {
	SID     core.SessionID
	Session core.MemberSession
}

func (r *Registry) MembersOfRoom(name domain.RoomName) []regSnap {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]regSnap, 0, len(r.sessions))
	for sid, e := range r.sessions {
		if e.RoomName == name {
			out = append(out, regSnap{SID: sid, Session: e.Session})
		}
	}
	return out
}

func (r *Registry) Cancel(sid core.SessionID) bool {
	r.mu.RLock()
	e, ok := r.sessions[sid]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	if e.Cancel != nil {
		e.Cancel()
	}
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Msg("canceled session")
	return true
}
