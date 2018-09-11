package main

import (
	"log"
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
func (p *Room) processConn(conn *ws.Conn, user string) {
	u := &User{
		conn: conn,
		key:  user,
	}

	p.Lock()
	p.Users[user] = u
	p.Unlock()

	go u.talker()
	defer p.cleanUp(u)

	for {
		var msg Msg
		err := conn.ReadJSON(&msg)
		if err != nil {
			return
		}
		log.Printf("Received message %v from %q in %q", msg, user, p.Key)

		// interpret message
		p.Lock()
		switch msg.Type {
		case msgTalk:
			if p.processCommands(u, msg.Msg) {
				p.Unlock()
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
			p.Paused = true
			p.updated = time.Now()
		case msgSelect:
			p.Watching = msg.Msg
			p.Paused = true
			p.Progress = 0
		case msgLoad:
			if _, ok := sets[msg.Msg]; ok {
				go func(set string) {
					err := p.toggleSet(set)
					if err != nil {
						log.Println("Error toggling set", err)
					}
					p.loadVideos()
				}(msg.Msg)
				msg.Data = ok
			}
		case msgPop:
			i := int(msg.Val)
			if i >= len(p.Queue) {
				break
			}

			p.Queue = append(p.Queue[:i], p.Queue[i+1:]...)
		case msgPush:
			if p.Watching == "" {
				msg.Type = msgSelect
				p.Watching = msg.Msg
				p.Paused = true
				p.Progress = 0
			} else {
				p.Queue = append(p.Queue, msg.Msg)
			}
		case msgNext:
			if len(p.Queue) == 0 {
				break
			}

			u.next = true
			ready := true
			for _, u := range p.Users {
				ready = ready && u.next
			}
			if ready {
				msg.Msg = msgStatus
				p.Watching = p.Queue[0]
				p.Queue = p.Queue[1:]
				p.Paused = false
				p.Progress = 0

				for _, u := range p.Users {
					u.next = false
				}
			}
		case msgReady:
			u.ready = true
			ready := true
			for _, u := range p.Users {
				ready = ready && u.ready
			}
			if ready {
				p.Unlock()
				continue
			} else {
				for _, u := range p.Users {
					u.ready = false
				}
			}
		}
		p.Unlock()

		// re-process message
		switch msg.Type {
		case msgTalk, msgPause, msgPlay, msgSeek, msgSelect, msgEvent, msgLoad:
			msg.From = user
			for _, u := range p.Users {
				u.msg <- msg
			}

			log.Printf("Sent message %v to %q", msg, p.Key)
		case msgPop, msgPush:
			msg.From = user
			p.notifyAll()
			fallthrough
		case msgStatus:
			p.notifyAll()
		}
	}
}
