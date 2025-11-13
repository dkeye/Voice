package core

import (
	"sync"

	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

// roomImpl is a threadsafe in-memory room.
// It never closes adapter-owned resources.
type roomImpl struct {
	room   *domain.Room
	mu     sync.RWMutex
	bySID  map[SessionID]MemberSession
	byUser map[domain.UserID]SessionID
}

func NewRoomService(room *domain.Room) RoomService {
	return &roomImpl{
		room:   room,
		bySID:  make(map[SessionID]MemberSession),
		byUser: make(map[domain.UserID]SessionID),
	}
}

func (r *roomImpl) Room() *domain.Room { return r.room }

func (r *roomImpl) MemberCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bySID)
}

func (r *roomImpl) AddMember(sid SessionID, ms MemberSession) {
	u := ms.Meta().User.ID
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bySID[sid] = ms
	r.byUser[u] = sid
	log.Info().Str("module", "core.room").Str("sid", string(sid)).Str("user", string(u)).Msg("member added")
}

func (r *roomImpl) RemoveMember(sid SessionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ms, ok := r.bySID[sid]; ok {
		u := ms.Meta().User.ID
		delete(r.byUser, u)
	}
	delete(r.bySID, sid)
	log.Info().Str("module", "core.room").Str("sid", string(sid)).Msg("member removed")
}

func (r *roomImpl) Broadcast(from SessionID, data Frame) PublishResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := PublishResult{}
	for sid, m := range r.bySID {
		if sid == from {
			continue
		}
		if err := m.Signal().TrySend(data); err != nil {
			res.Dropped = append(res.Dropped, m)
			continue
		}
		res.SendTo++
	}
	log.Debug().Str("module", "core.room").Str("from", string(from)).Int("sent_to", res.SendTo).Int("dropped", len(res.Dropped)).Msg("broadcast result")
	return res
}

func (r *roomImpl) MembersSnapshot() []MemberDTO {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MemberDTO, 0, len(r.bySID))
	for _, ms := range r.bySID {
		u := ms.Meta().User
		out = append(out, MemberDTO{ID: u.ID, Username: u.Username})
	}
	return out
}
