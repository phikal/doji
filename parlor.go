package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// id -> parlor map and global lock
	parlors = make(map[string]*Parlor)
	lock    = &sync.Mutex{}
)

// a parlor (video-room, session) stores the current state of the room,
// as well as the users (in a map) and further internal data
type Parlor struct {
	Paused   bool             // is current video paused
	Progress float64          // seconds into current video
	Users    map[string]*User // users (id -> User) in this parlor
	Videos   []string         // videos available to be selected
	Watching string           // video currently being watched
	Key      string           // parlors key pointing to this Parlor
	Dlds     []Request        // List of current downloads
	lock     *sync.Mutex      // lock for .Users map
	updated  time.Time        // last status update recived
	notif    chan<- *User     // send a status update to this user
	format   string           // ytdl-format (-f) to use
	reqs     chan bool        // request coordinator
}

func (p *Parlor) processCommands(u *User, msg string) bool {
	fields := strings.Fields(msg)

	if len(fields) < 1 {
		return false
	}

	hasArg := len(fields) > 1
	switch fields[0] {
	case "/stats", "/stat":
		fi, err := ioutil.ReadDir(p.Key)
		if err != nil {
			u.msg <- Msg{
				Type: msgEvent,
				Msg:  fmt.Sprintf("Failed to read files: %s", err),
			}
			break
		}

		var sum int64
		for _, f := range fi {
			sum += f.Size()
		}

		u.msg <- Msg{
			Type: msgReqst,
			Msg:  fmt.Sprintf("all files require %.4gMiB", sum/1024/1024),
		}
	case "/delete":
		if hasArg {
			for i, f := range p.Videos {
				if f == fields[1] {
					err := os.Remove(path.Join(p.Key, f))
					if err != nil {
						u.msg <- Msg{
							Type: msgEvent,
							Msg:  fmt.Sprintf("Failed to delete %s: %s", f, err),
						}
						break
					}

					p.Videos = append(p.Videos[:i], p.Videos[i+1:]...)
					for _, u := range p.Users {
						p.notif <- u
					}
				}
			}
			p.loadVideos()
		}
	case "/update":
		p.loadVideos()
	case "/format":
		if hasArg {
			p.format = fields[1]
		}
		u.msg <- Msg{
			Type: msgReqst,
			Msg:  fmt.Sprintf("set format to %q", p.format),
		}
	}
	return strings.HasPrefix(fields[0], "/")
}

// check for new videos in the parlor's directory
func (p *Parlor) loadVideos() {
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
	for _, u := range p.Users {
		p.notif <- u
	}
}

// wait for requests to send users the current status
func (p *Parlor) statusMonitor() {
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
		p.lock.Lock()
		msg.Data = map[string]interface{}{
			"vids":     p.Videos,
			"users":    p.Users,
			"paused":   p.Paused,
			"playing":  p.Watching,
			"progress": p.Progress,
			"download": p.Dlds
		}

		if user.msg != nil {
			user.msg <- msg
		} else {
			notif <- user
			time.Sleep(time.Millisecond * 50)
		}
		p.lock.Unlock()
	}
}

// clean up after a user, and if the parlor stays empty, delete it too
func (p *Parlor) cleanUp(user *User) {
	for k, u := range p.Users {
		if u != user {
			p.notif <- u
		} else {
			p.lock.Lock()
			delete(p.Users, k)
			p.lock.Unlock()
		}
	}

	time.Sleep(5 * time.Minute)
	if len(p.Users) == 0 {
		lock.Lock()
		delete(parlors, p.Key)
		lock.Unlock()
		if err := os.RemoveAll(p.Key); err != nil {
			log.Panicln(err)
		}
	}
}
