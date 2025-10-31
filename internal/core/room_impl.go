package core

import (
	"sync"

	"github.com/dkeye/Voice/internal/domain"
)

// roomImpl is a threadsafe in-memory room.
// It never closes adapter-owned resources.
type roomImpl struct {
	room    *domain.Room
	mu      sync.RWMutex
	members map[domain.UserID]MemberSession
}

func NewRoomService(room *domain.Room) RoomService {
	return &roomImpl{
		room:    room,
		members: make(map[domain.UserID]MemberSession),
	}
}

func (r *roomImpl) Room() *domain.Room { return r.room }

func (r *roomImpl) MemberCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.members)
}

func (r *roomImpl) AddMember(ms MemberSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := ms.Meta().User.ID
	r.members[id] = ms
}

func (r *roomImpl) RemoveMember(id domain.UserID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.members, id)
}

func (r *roomImpl) Broadcast(from domain.UserID, data Frame) PublishResult {
	r.mu.RLock()
	// Copy recipients under read lock to avoid holding the lock during sends.
	targets := make([]MemberSession, 0, len(r.members))
	for id, ms := range r.members {
		if id == from {
			continue
		}
		targets = append(targets, ms)
	}
	r.mu.RUnlock()

	var res PublishResult
	for _, ms := range targets {
		if err := ms.Conn().TrySend(data); err != nil {
			res.Dropped = append(res.Dropped, ms)
		} else {
			res.SendTo++
		}
	}
	return res
}

func (r *roomImpl) MembersSnapshot() []MemberDTO {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MemberDTO, 0, len(r.members))
	for _, ms := range r.members {
		u := ms.Meta().User
		out = append(out, MemberDTO{ID: u.ID, Username: u.Username})
	}
	return out
}
