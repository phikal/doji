package main

import (
	"html"
	"log"
	"math/rand"
	"sync/atomic"

	ws "github.com/gorilla/websocket"
)

var idCounter uint32

// Msg stores websocket messages, which are sent from and to the client
type Msg struct {
	Type string      `json:"type"`
	Msg  string      `json:"msg,omitempty"`
	Val  float64     `json:"val,omitempty"`
	From string      `json:"from,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

func (p *Room) processMsg(msg Msg, user *User) {
	p.Lock()
	defer p.Unlock()

	switch msg.Type {
	case msgTalk:
		if p.processCommands(user, msg.Msg) {
			break
		}
		msg.Msg = html.EscapeString(msg.Msg)
	case msgPlay:
		p.update(msg.Val, false)
	case msgPause, msgSeek:
		p.update(msg.Val, true)
	case msgSelect:
		set, ok := p.Sets[msg.Msg]
		if ok {
			p.Watching = (*set)[int(msg.Val)]
			p.update(0, true)
		}
	case msgLoad:
		if _, ok := sets[msg.Msg]; ok {
			p.toggleSet(msg.Msg)
			msg.Data = ok
		}
	case msgPop:
		i := int(msg.Val)
		if i >= len(p.Queue) {
			break
		}

		p.Queue = append(p.Queue[:i], p.Queue[i+1:]...)
	case msgPush:
		set, ok := p.Sets[msg.Msg]
		if !ok {
			break
		}

		video := (*set)[int(msg.Val)]
		if p.Watching == nil {
			msg.Type = msgSelect
			p.Watching = video
			p.update(0, true)
		} else {
			p.Queue = append(p.Queue, video)
		}
	case msgNext:
		if len(p.Queue) == 0 {
			break
		}

		user.next = true
		ready := true
		for _, user := range p.Users {
			ready = ready && user.next
		}

		if ready {
			msg.Type = msgSelect
			msg.Data = p.Queue[0]
			p.Watching = p.Queue[0]
			p.Queue = p.Queue[1:]
			p.update(0, true)

			for _, user := range p.Users {
				user.next = false
			}
		}
	case msgReady:
		user.ready = true
		ready := true
		for _, user := range p.Users {
			ready = ready && user.ready
		}
		if !ready {
			msg.Type = msgPlay
			msg.Val = p.Progress()

			for _, user := range p.Users {
				user.ready = false
			}
		}
	}
}

// continuously listen on a connection and process incoming messages, as
// well as start a goroutine to coordinate outgoing messages
func (p *Room) processConn(conn *ws.Conn, userName string) {
	user := &User{
		id:   atomic.AddUint32(&idCounter, rand.Uint32()%(1<<12)),
		conn: conn,
		key:  userName,
	}

	p.Lock()
	if _, ok := p.Users[userName]; ok {
		conn.Close()
		p.Unlock()
		return
	}

	p.Users[userName] = user
	p.Unlock()

	go user.talker()
	go p.cleaner()

	defer func() {
		p.clear <- user
	}()

	for {
		msg := Msg{Val: -1}
		err := conn.ReadJSON(&msg)
		if err != nil {
			return
		}
		log.Printf("Received message %#v from %q in %q", msg, user.id, p.Key)

		// interpret message
		p.processMsg(msg, user)

		// re-process message
		switch msg.Type {
		case msgPop, msgPush:
			msg.From = string(user.id)
			p.notifyAll()
			fallthrough
		case msgTalk, msgPause, msgPlay, msgSeek, msgSelect, msgEvent, msgLoad:
			msg.From = string(user.id)
			for _, user := range p.Users {
				user.msg <- msg
			}

			log.Printf("Sent message %v to %q", msg, p.Key)
		case msgStatus:
			p.notifyAll()
		}
	}
}
