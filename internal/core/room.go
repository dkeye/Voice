package core

type Message struct {
	Sender *Client
	Data []byte
}

type Room struct {
	Name       string
	Clients    map[*Client]bool
	Broadcast  chan Message
	Register   chan *Client
	Unregister chan *Client
}

func NewRoom(name string) *Room {
	return &Room{
		Name:       name,
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (r *Room) Run() {
	for {
		select {
		case c := <-r.Register:
			r.Clients[c] = true
		case c := <-r.Unregister:
			if _, ok := r.Clients[c]; ok {
				delete(r.Clients, c)
				close(c.Send)
			}
		case msg := <-r.Broadcast:
			for c := range r.Clients {
				if c == msg.Sender {
					continue
				}
				select {
				case c.Send <- msg.Data:
				default:
					close(c.Send)
					delete(r.Clients, c)
				}
			}
		}
	}
}
