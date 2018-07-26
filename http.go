package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"

	ws "github.com/gorilla/websocket"
	"strings"
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

	// path regular expression
	pathRe = regexp.MustCompile("^/([" + rchars + "]+)")
)

func createParlor(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id := r.PostFormValue("name")
	if _, ok := parlors[id]; !ok {
		id = rndName()
		lock.Lock()
		parlors[id] = &Parlor{
			Users:  make(map[string]*User),
			Key:    id,
			lock:   &sync.Mutex{},
			format: "best",
		}
		lock.Unlock()

		if err := os.Mkdir(id, os.ModeDir|0755); err != nil {
			log.Fatalln(err)
		}

		go parlors[id].statusMonitor()
	}

	http.Redirect(w, r, "./"+id, http.StatusFound)

}

func connect(name string, w http.ResponseWriter, r *http.Request) {
	p, ok := parlors[name]
	if !ok {
		http.Error(w, "no such parlor", http.StatusBadRequest)
		return
	}

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

	p.processConn(conn, user)
}

// http handler for creating parlors and websockets
func parlor(w http.ResponseWriter, r *http.Request) {
	// create or join existing parlor
	if r.Method == http.MethodPost {
		createParlor(w, r)
	} else {
		m := pathRe.FindStringSubmatch(r.URL.Path)
		if len(m) == 2 {
			if strings.HasSuffix(r.URL.Path, "/socket") {
				connect(m[1], w, r)
			} else if err := tmpl.ExecuteTemplate(w, "parlor", parlors[m[1]]); err != nil {
				log.Fatal(err)
			}
		}
	}
}
