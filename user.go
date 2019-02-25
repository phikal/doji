package main

import (
	"log"

	ws "github.com/gorilla/websocket"
)

// User represents, and contains, a websocket connection
type User struct {
	ID   uint32 `json:"id"`   // internal ID for a user per room
	Name string `json:"name"` // public username

	next  bool       // has finished watching current video
	ready bool       // is ready to play next video
	oper  bool       // is user an operator
	key   string     // verification key
	conn  *ws.Conn   // socket connection for a user
	msg   chan<- Msg // channel for sending messages to this user
}

func (u *User) talker() {
	c := make(chan Msg, 16)
	u.msg = c
	for {
		data := <-c
		log.Printf("Sent message to %x [%s]: %v", u.ID, u.Name, data)
		if err := u.conn.WriteJSON(data); err != nil {
			log.Println(err)
		}
	}
}
