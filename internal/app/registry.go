package app

import (
	"context"
	"sync"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
)

type sessionEntry struct {
	UserID   domain.UserID
	RoomName domain.RoomName
	Session  core.MemberSession
	Cancel   context.CancelFunc
}

type Registry struct {
	mu   sync.RWMutex
	sess map[core.SessionID]sessionEntry
}

func NewRegistry() *Registry {
	return &Registry{
		sess: make(map[core.SessionID]sessionEntry),
	}
}

func (r *Registry) BindSession(
	sid core.SessionID,
	user domain.UserID,
	roomName domain.RoomName,
	sess core.MemberSession,
	cancel context.CancelFunc,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sess[sid] = sessionEntry{
		UserID:   user,
		RoomName: roomName,
		Session:  sess,
		Cancel:   cancel,
	}
}

func (r *Registry) Unbind(sid core.SessionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sess, sid)
}

func (r *Registry) RoomOf(sid core.SessionID) (domain.RoomName, domain.UserID, core.MemberSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.sess[sid]
	if !ok {
		return "", "", nil, false
	}
	return entry.RoomName, entry.UserID, entry.Session, true
}

func (r *Registry) UpdateRoom(sid core.SessionID, newRoom domain.RoomName) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.sess[sid]
	if !ok {
		return false
	}
	entry.RoomName = newRoom
	r.sess[sid] = entry
	return true
}

type regSnap struct {
	SID     core.SessionID
	Session core.MemberSession
}

func (r *Registry) MembersOfRoom(name domain.RoomName) []regSnap {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]regSnap, 0, len(r.sess))
	for sid, e := range r.sess {
		if e.RoomName == name {
			out = append(out, regSnap{SID: sid, Session: e.Session})
		}
	}
	return out
}

func (r *Registry) Cancel(sid core.SessionID) bool {
	r.mu.RLock()
	e, ok := r.sess[sid]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	if e.Cancel != nil {
		e.Cancel()
	}
	return true
}
