package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"sync"
	"time"
)

var (
	// id -> room map and global lock
	rooms = make(map[string]*Room)
	lock  = &sync.RWMutex{}
)

// a room stores the current state of the room, as well as the users (in
// a map) and further internal data
type Room struct {
	sync.RWMutex // lock for .Users map

	Key      string           // room's key pointing to this Parlor
	Paused   bool             // is current video paused
	Progress float64          // seconds into current video
	Users    map[string]*User // users (id -> User) in this room
	Requests []*Request       // videos currently being downloaded
	Videos   []string         // lits of all videos currently in this rooms
	Queue    []string         // list of videos to be played after current one
	Sets     map[string]int   // map (id -> size) of all loaded sets
	Watching string           // video currently being watched
	QueueReq bool             // should new requests be queued?
	format   string           // ytdl-format (-f) to use
	notif    chan<- *User     // send a status update to this user
	reqs     chan bool        // request coordinator
	updated  time.Time        // last status update recived
func (p *Room) notifyAll() {
	for _, u := range p.Users {
		p.notif <- u
	}
}

// check for new videos in the room's directory
func (p *Room) loadVideos() {
	p.Videos = nil

	files, err := ioutil.ReadDir(p.Key)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		switch path.Ext(file.Name()) {
		case ".mp4", ".webm", ".mkv", ".avi", ".3gp":
			p.Videos = append(p.Videos, file.Name())
		}
	}

	sort.Strings(p.Videos)
	p.notifyAll()
}

// wait for requests to send users the current status
func (p *Room) statusMonitor() {
	var msg Msg
	msg.Type = msgStatus
	notif := make(chan *User, 4)
	p.notif = notif

	for {
		user := <-notif

		if !p.Paused {
			p.Progress += time.Since(p.updated).Seconds()
			p.updated = time.Now()
		}
		p.Lock()
		msg.Data = map[string]interface{}{
			"vids":     p.Videos,
			"sets":     setNames,
			"lsets":    p.Sets,
			"queue":    p.Queue,
			"reqs":     p.Requests,
			"users":    p.Users,
			"paused":   p.Paused,
			"playing":  p.Watching,
			"progress": p.Progress,
		}

		if user.msg != nil {
			user.msg <- msg
		} else {
			notif <- user
			time.Sleep(time.Millisecond * 50)
		}
		p.Unlock()
	}
}

// clean up after a user, and if the room stays empty, delete it too
func (p *Room) cleanUp(user *User) {
	for k, u := range p.Users {
		if u != user {
			p.notif <- u
		} else {
			p.Lock()
			delete(p.Users, k)
			p.Unlock()
		}
	}

	time.Sleep(5 * time.Minute)
	if len(p.Users) == 0 {
		lock.Lock()
		delete(rooms, p.Key)
		lock.Unlock()
		if err := os.RemoveAll(p.Key); err != nil {
			log.Panicln(err)
		}
	}
}
