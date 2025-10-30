package core

import (
	"context"
	"sync"
)

type RoomManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	rooms  map[string]*Room
}

func NewRoomManager(parent context.Context) *RoomManager {
	ctx, cancel := context.WithCancel(parent)

	return &RoomManager{
		ctx:    ctx,
		cancel: cancel,
		mu:     sync.RWMutex{},
		rooms:  make(map[string]*Room),
	}
}

func (rm *RoomManager) GetOrCreate(name string) *Room {
	rm.mu.RLock()
	room, ok := rm.rooms[name]
	rm.mu.RUnlock()

	if ok {
		return room
	}

	rm.mu.Lock()
	if room, ok = rm.rooms[name]; !ok {
		roomCtx, roomCancel := context.WithCancel(rm.ctx)
		room = NewRoom(roomCtx, roomCancel, name)
		rm.rooms[name] = room
		go room.Run()
	}
	rm.mu.Unlock()
	return room
}

func (rm *RoomManager) List() []RoomsInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	rl := make([]RoomsInfo, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		rl = append(rl, r.GetInfo())
	}
	return rl
}

func (rm *RoomManager) StopRoom(name string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if room, ok := rm.rooms[name]; ok {
		room.Stop()
		delete(rm.rooms, room.Name)
	}
}
