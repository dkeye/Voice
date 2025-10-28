package core

import "log"

type Client struct {
	Name string
	Room *Room
}

func NewClient(name string) *Client {
	c := &Client{
		Name: name,
	}
	return c
}

func (c *Client) Send(data []byte) {
	log.Printf("client [%s] will recieve message %s", c.Name, data)
}
