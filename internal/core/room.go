package core

import (
	"sync"
)

type Room struct {
	Name    string
	Clients map[string]*Client
	mu      sync.Mutex
}

func NewRoom(name string) *Room {
	return &Room{
		Name:    name,
		Clients: make(map[string]*Client),
	}
}

func (r *Room) AddClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Clients[c.Name] = c
}

func (r *Room) RemoveClient(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Clients, name)
}

func (r *Room) Broadcast(sender *Client, data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, client := range r.Clients {
		client.Send(data)
	}
}
