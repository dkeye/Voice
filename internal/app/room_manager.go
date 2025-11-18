package app

import (
	"sync"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

type RoomManagerImpl struct {
	mu    sync.RWMutex
	rooms map[domain.RoomID]core.RoomService
}

func NewRoomManager() core.RoomManager {
	return &RoomManagerImpl{rooms: make(map[domain.RoomID]core.RoomService)}
}

func (f *RoomManagerImpl) CreateRoom(name domain.RoomName) core.RoomService {
	room := core.NewRoomService(domain.RoomName(name))
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rooms[room.Room().ID] = room
	log.Info().Str("module", "app.roommgr").Str("room_id", string(room.Room().ID)).Str("room_name", string(room.Room().Name)).Msg("created room")
	return room
}

func (f *RoomManagerImpl) GetRoom(id domain.RoomID) (core.RoomService, bool) {
	f.mu.RLock()
	room, ok := f.rooms[id]
	f.mu.RUnlock()
	return room, ok
}

func (f *RoomManagerImpl) List() []core.RoomInfo {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]core.RoomInfo, 0, len(f.rooms))
	for id, r := range f.rooms {
		out = append(out, core.RoomInfo{ID: id, Name: r.Room().Name, MemberCount: r.MemberCount()})
	}
	return out
}

func (f *RoomManagerImpl) StopRoom(id domain.RoomID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rooms, id)
	log.Info().Str("module", "app.roommgr").Str("room_id", string(id)).Msg("stopped room")
}
