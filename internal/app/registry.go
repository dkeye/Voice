package app

import (
	"context"
	"sync"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

type sessionEntry struct {
	RoomID  domain.RoomID
	Session core.MemberSession
	Cancel  context.CancelFunc
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

func (r *Registry) GetOrCreateUser(sid core.SessionID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.users[sid]; ok {
		return u, nil
	}
	u, err := domain.NewUser("guest")
	if err != nil {
		return nil, err
	}
	r.users[sid] = u
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Msg("created new user")
	return u, nil
}

func (r *Registry) UpdateUsername(sid core.SessionID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.users[sid]; ok {
		err := u.SetUsername(name)
		if err != nil {
			log.Error().Err(err).Str("module", "app.registry").Str("sid", string(sid)).Msg("failed to update username")
			return err
		}
		log.Info().
			Str("module", "app.registry").
			Str("sid", string(sid)).
			Str("name", name).
			Msg("username updated")
	}
	return nil
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
	if e, ok := r.sessions[sid]; ok {
		if e.Cancel != nil {
			e.Cancel()
		}
		delete(r.sessions, sid)
	}
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Msg("unbind session")
}

func (r *Registry) RoomOf(sid core.SessionID) (domain.RoomID, core.MemberSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.sessions[sid]
	if !ok || entry.RoomID == "" {
		return "", nil, false
	}
	return entry.RoomID, entry.Session, true
}

func (r *Registry) UpdateRoom(sid core.SessionID, newRoom domain.RoomID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.sessions[sid]
	if !ok {
		return false
	}
	entry.RoomID = newRoom
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Str("room", string(newRoom)).Msg("updated room")
	return true
}

func (r *Registry) RemoveRoom(sid core.SessionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.sessions[sid]; ok {
		entry.RoomID = ""
	}
	log.Info().Str("module", "app.registry").Str("sid", string(sid)).Msg("removed room association")
}

type regSnap struct {
	SID     core.SessionID
	Session core.MemberSession
}

func (r *Registry) MembersOfRoom(id domain.RoomID) []regSnap {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]regSnap, 0, len(r.sessions))
	for sid, e := range r.sessions {
		if e.RoomID == id {
			out = append(out, regSnap{SID: sid, Session: e.Session})
		}
	}
	return out
}

func (r *Registry) RoomMates(sid core.SessionID) (res []regSnap) {
	roomID, _, ok := r.RoomOf(sid)
	if ok {
		res = r.MembersOfRoom(roomID)
	}
	return
}
