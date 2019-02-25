package main

import "time"

// wait for requests to send users the current status
func (p *Room) status() {
	var msg Msg
	msg.Type = msgStatus
	notif := make(chan *User, 4)
	p.notif = notif

	for {
		user := <-notif

		msg.Data = map[string]interface{}{
			"sets":     sets,
			"loaded":   p.Sets,
			"queue":    p.Queue,
			"users":    p.Users,
			"paused":   p.paused,
			"playing":  p.Watching,
			"progress": p.Progress(),
		}

		if user.msg != nil {
			user.msg <- msg
		} else {
			notif <- user
			time.Sleep(time.Millisecond * 50)
		}
	}
}
