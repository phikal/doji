package main

import (
	"time"

	ws "github.com/gorilla/websocket"
)

// Msg stores websocket messages, which are sent from and to the client
type Msg struct {
	Type string      `json:"type"`
	Msg  string      `json:"msg,omitempty"`
	Val  float64     `json:"val,omitempty"`
	From string      `json:"from,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// continuously listen on a connection and process incoming messages, as
// well as start a goroutine to coordinate outgoing messages
func (p *Parlor) processConn(conn *ws.Conn, user string) {
	u := &User{conn: conn}

	p.lock.Lock()
	p.Users[user] = u
	p.lock.Unlock()

	go u.talker()
	defer p.cleanUp(u)

	var msg Msg
	for {
		err := conn.ReadJSON(&msg)
		if err != nil {
			return
		}

		// interpret message
		p.lock.Lock()
		switch msg.Type {
		case msgTalk:
			if p.processCommands(u, msg.Msg) {
				p.lock.Unlock()
				continue
			}
		case msgPlay:
			p.Paused = false
			p.Progress = msg.Val
			p.updated = time.Now()
		case msgPause:
			p.Paused = true
			p.Progress = msg.Val
		case msgSeek:
			p.Progress = msg.Val
			p.updated = time.Now()
		case msgSelect:
			p.Watching = msg.Msg
			p.Paused = true
			p.Progress = 0
		case msgReqst:
			go p.getVideo(msg.Msg)
		}
		p.lock.Unlock()

		// re-process message
		switch msg.Type {
		case msgTalk, msgPause, msgPlay, msgSeek, msgSelect:
			msg.From = user
			for _, u := range p.Users {
				u.msg <- msg
			}
		case msgStatus:
			for _, u := range p.Users {
				p.notif <- u
			}
		}
	}
}
