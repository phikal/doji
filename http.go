package main

import (
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"

	ws "github.com/gorilla/websocket"
)

var (
	// Go-template engine
	tmpl = template.New("")

	// websocket upgraded with custom origin-checker
	upgrader = ws.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // suboptimal
		},
	}

	// regular expression for paths matching room URLs
	roomRe = regexp.MustCompile("^[a-z]{2,}")
)

func create(room string) *Room {
	var R *Room
	var ok bool

	if R, ok = rooms[room]; !ok {
		R = &Room{
			Users:  make(map[string]*User),
			Sets:   make(map[string]int),
			Key:    room,
			format: "best",
			reqs:   make(chan bool, MAX_PROCS),
		}

		for i := 0; i < MAX_PROCS; i++ {
			R.reqs <- true
		}

		if err := os.Mkdir(room, os.ModeDir|0755); err != nil && !os.IsExist(err) {
			log.Fatalln(err)
		}

		lock.Lock()
		rooms[room] = R
		lock.Unlock()

		go R.statusMonitor()
	}

	return R
}

func connect(w http.ResponseWriter, r *http.Request, room *Room) {
	cookie, err := r.Cookie("user")
	if err != nil {
		http.Error(w, "no username passed", http.StatusBadRequest)
		return
	}
	user := cookie.Value

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
		return
	}

	room.processConn(conn, user)
}

// http handler for creating rooms and websockets
func room(w http.ResponseWriter, r *http.Request) {
	room := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case r.URL.Path == "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "room", nil); err != nil {
			log.Fatal(err)
		}
	case strings.HasSuffix(r.URL.Path, "/socket"):
		room = strings.TrimSuffix(room, "/socket")
		if p, ok := rooms[room]; !ok {
			http.Error(w, "no such room", http.StatusBadRequest)
		} else {
			connect(w, r, p)
		}
	case roomRe.MatchString(room):
		log.Printf("%q joined room %q", "user", room)
		p := create(room)
		if err := tmpl.ExecuteTemplate(w, "room", p); err != nil {
			log.Fatal(err)
		}
	}
}
