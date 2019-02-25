package main

import (
	"log"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// id -> room map and global lock
	rooms = make(map[string]*Room)
	lock  sync.Mutex
)

// Room stores the current state of the room, as well as the users (in
// a map) and further internal data
type Room struct {
	sync.Mutex // lock for .Users/.Sets map

	// room's key pointing to this Parlor
	Key string

	// map (id -> User) of users in this room
	Users map[uint32]*User

	// list of videos in this room
	Videos []*Video

	// list of videos to be played after
	// current one (Queue ⊆ Videos)
	Queue []*Video

	// list of loaded sets
	Sets []*Set

	// video currently being watched
	Watching *Video

	format  string        // ytdl-format (-f) to use
	notif   chan<- *User  // send a status update to this user
	clear   chan<- *User  // informs server to clean up after a user
	progrs  time.Duration // seconds into current video
	paused  bool          // is current video paused
	locked  bool          // can only OPs change room state
	updated time.Time     // last status update received
	state   sync.Mutex    // lock to change room state
	gencnt  uint32        // generation counter for room-state changes
}

func create(room string) *Room {
	p, ok := rooms[room]

	if !ok {
		p = &Room{
			Users:  make(map[uint32]*User),
			Key:    room,
			format: "best",
		}

		err := os.Mkdir(path.Join(pwd, room), os.ModeDir|0755)
		if err != nil {
			log.Println(err)
			return nil
		}

		lock.Lock()
		rooms[room] = p
		lock.Unlock()

		go p.cleaner()
		go p.status()
	}

	return p
}

func (p *Room) update(progress time.Duration, paused bool) {
	gen := p.gencnt

	p.state.Lock()
	if atomic.CompareAndSwapUint32(&p.gencnt, gen, p.gencnt+1) {
		p.paused = paused
		p.updated = time.Now()

		time.Sleep(time.Millisecond * 250)
	}
	p.state.Unlock()
}

func (p *Room) progress() time.Duration {
	if p.paused {
		return p.progrs
	}

	return p.progrs + time.Since(p.updated)
}

func (p *Room) notifyAll() {
	for _, u := range p.Users {
		p.notif <- u
	}
}

// clean up after a user, and if the room stays empty, delete it too
func (p *Room) cleaner() {
	clear := make(chan *User, 16)
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
			// if room is still empty after 30 seconds, delete it
			time.Sleep(time.Second * 30)
			if len(p.Users) == 0 {
				break
			}
		}
	}

	lock.Lock()
	delete(rooms, p.Key)
	lock.Unlock()
	if err := os.RemoveAll(p.Key); err != nil {
		log.Panicln(err)
	}

	for _, s := range p.Sets {
		for _, v := range s.content {
			if v.cmd != nil {
				v.cmd.Process.Kill()
			}
		}
	}
}
