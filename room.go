package main

import (
	"log"
	"os"
	"sync"
	"time"
	"path"
)

var (
	// id -> room map and global lock
	rooms = make(map[string]*Room)
	lock  = &sync.Mutex{}
)

// Room stores the current state of the room, as well as the users (in
// a map) and further internal data
type Room struct {
	sync.Mutex // lock for .Users map

	Key      string           // room's key pointing to this Parlor
	Users    map[string]*User // users (id -> User) in this room
	Queue    []*Video         // list of videos to be played after current one
	Sets     map[string]*Set  // map (id -> size) of all loaded sets
	Watching *Video           // video currently being watched

	format   string       // ytdl-format (-f) to use
	notif    chan<- *User // send a status update to this user
	clear    chan<- *User // informs server to clean up after a user
	progress float64      // seconds into current video
	paused   bool         // is current video paused
	updated  time.Time    // last status update received
}

func create(room string) *Room {
	var R *Room
	var ok bool

	if R, ok = rooms[room]; !ok {
		R = &Room{
			Users:  make(map[string]*User),
			Sets:   make(map[string]*Set),
			Key:    room,
			format: "best",
		}

		err := os.Mkdir(path.Join(pwd, room), os.ModeDir|0755)
		if err != nil {
			log.Fatalln(err)
		}

		lock.Lock()
		rooms[room] = R
		lock.Unlock()

		go R.status()
	}

	return R
}

func (p *Room) update(progress float64, paused bool) {
	p.progress = progress
	p.paused = paused
	p.updated = time.Now()
}

func (p *Room) Progress() float64 {
	if p.paused {
		return p.progress
	} else {
		return p.progress + time.Since(p.updated).Seconds()
	}
}

func (p *Room) notifyAll() {
	for _, u := range p.Users {
		p.notif <- u
	}
}

// clean up after a user, and if the room stays empty, delete it too
func (p *Room) cleaner() {
	clear := make(chan *User, 4)
	p.clear = clear
	for {
		user := <-clear
		for k, u := range p.Users {
			if u != user {
				p.notif <- u
			} else {
				p.Lock()
				delete(p.Users, k)
				p.Unlock()
			}
		}

		if len(p.Users) == 0 {
			break
		}
	}

	lock.Lock()
	delete(rooms, p.Key)
	lock.Unlock()
	if err := os.RemoveAll(p.Key); err != nil {
		log.Panicln(err)
	}

	for _, s := range p.Sets {
		for _, v := range *s {
			if v.cmd != nil {
				v.cmd.Process.Kill()				
			}
		}
	}

}
