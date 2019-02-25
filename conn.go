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
	Val  int64       `json:"val,omitempty"`
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
		p.update(time.Duration(msg.Val), false)
	case msgPause, msgSeek:
		p.update(time.Duration(msg.Val), true)
	case msgSelect:
		if msg.Val >= 0 && msg.Val < int64(len(p.Videos)) {
			p.Watching = p.Videos[msg.Val]
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
		if msg.Val >= 0 && msg.Val < int64(len(p.Videos)) {
			if p.Watching == nil {
				msg.Type = msgSelect
				p.Watching = p.Videos[msg.Val]
				p.update(0, true)
			} else {
				p.Queue = append(p.Queue, p.Videos[msg.Val])
			}
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
			msg.Val = int64(p.progress())

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
		ID:   atomic.AddUint32(&idCounter, rand.Uint32()%(1<<10)),
		Name: userName,
		key:  fmt.Sprintf("%x", rand.Uint32()%(1<<22)+(1<<10)),
		conn: conn,
	}

	p.Lock()
	p.Users[user.ID] = user
	p.Unlock()

	go user.talker()

	defer conn.Close()

	for {
		msg := Msg{Val: -1}
		err := conn.ReadJSON(&msg)
		if err != nil {
			if err != ws.ErrCloseSent {
				log.Printf("socket %d [%s] encountered fatal error: %s",
					user.ID, userName, err)
			}
			p.clear <- user
			return
		}
		log.Printf("Received message %#v from %q in %q", msg, user.Name, p.Key)

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
