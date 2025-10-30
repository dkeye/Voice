package core

import (
	"context"
	"sync"
)

type Message struct {
	Sender *Client
	Data   []byte
}

type RoomsInfo struct {
	Name         string `json:"name"`
	ClientsCount int    `json:"client_count"`
}

type Room struct {
	Name string

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.RWMutex
	clients map[*Client]bool

	broadcast  chan Message
}

func (r *Room) GetClientsCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (r *Room) GetInfo() RoomsInfo {
	return RoomsInfo{
		Name:         r.Name,
		ClientsCount: r.GetClientsCount(),
	}
}

func NewRoom(ctx context.Context, cancel context.CancelFunc, name string) *Room {
	return &Room{
		Name:       name,
		ctx:        ctx,
		cancel:     cancel,
		mu:         sync.RWMutex{},
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message),
	}
}

func (r *Room) AddClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = true
}

func (r *Room) RemoveClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clients[c]; ok {
		c.Close()
		delete(r.clients, c)
	}
}

func (r *Room) Run() {
	defer r.cleanup()
	for {
		select {
		case <-r.ctx.Done():
			return
		case msg := <-r.broadcast:
			r.mu.RLock()
			for c := range r.clients {
				if c == msg.Sender {
					continue
				}
				select {
				case c.Send <- msg.Data:
				// if c.Send already close
				default:
					r.RemoveClient(c)
				}
			}
			r.mu.RUnlock()
		}
	}
}

func (r *Room) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		close(c.Send)
		delete(r.clients, c)
	}
	close(r.broadcast)
}

func (r *Room) Stop() {
	r.cancel()
}