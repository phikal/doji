package main

import ws "github.com/gorilla/websocket"

// User represents, and contains, a websocket connection
type User struct {
	key     string
	conn    *ws.Conn
	ignored bool
	next    bool
	ready   bool
	msg     chan<- Msg
}

func (u *User) talker() {
	c := make(chan Msg, 16)
	u.msg = c
	for {
		u.conn.WriteJSON(<-c)
	}
}
