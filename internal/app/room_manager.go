package app

import (
	"sync"

	"github.com/dkeye/Voice/internal/core"
	"github.com/dkeye/Voice/internal/domain"
)

type RoomFactoryImpl struct {
	mu    sync.RWMutex
	rooms map[domain.RoomName]core.RoomService
}

func NewRoomManager() core.RoomFactory {
	return &RoomFactoryImpl{rooms: make(map[domain.RoomName]core.RoomService)}
}

func (f *RoomFactoryImpl) GetOrCreate(name domain.RoomName) core.RoomService {
	f.mu.RLock()
	room, ok := f.rooms[name]
	f.mu.RUnlock()
	if ok {
		return room
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if room, ok = f.rooms[name]; ok {
		return room
	}
	room = core.NewRoomService(&domain.Room{Name: name})
	f.rooms[name] = room
	return room
}

func (f *RoomFactoryImpl) List() []core.RoomInfo {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]core.RoomInfo, 0, len(f.rooms))
	for name, r := range f.rooms {
		out = append(out, core.RoomInfo{Name: name, MemberCount: r.MemberCount()})
	}
	return out
}

func (f *RoomFactoryImpl) StopRoom(name domain.RoomName) {
	f.mu.Lock()
	delete(f.rooms, name)
	f.mu.Unlock()
}
