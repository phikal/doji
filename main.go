package main

//go:generate go-bindata -o templ.go static/...

import (
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// message types
	msgTalk   = "talk"
	msgPause  = "pause"
	msgPlay   = "play"
	msgSeek   = "seek"
	msgSelect = "select"
	msgStatus = "status"
	msgReqst  = "request"
	msgEvent  = "event"
	msgLoad   = "load"
	msgPop    = "pop"
	msgPush   = "push"
	msgNext   = "next"
	msgReady  = "ready"

	pwd = "/tmp/doji"
)

var setDir string

func sigCatch() {
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill, syscall.SIGUSR1)
	ticker := time.Tick(time.Minute * 30)

	for {
		var s os.Signal
		select {
		case s = <-sig:
		case <-ticker:
		}

		if s == syscall.SIGUSR1 {
			err := initSets()
			if err != nil {
				log.Fatalln(err)
			}
		} else {
			break
		}
	}

	err := os.RemoveAll(pwd)
	if err != nil {
		log.Fatalln(err)
	}

	time.Sleep(time.Millisecond * 500)
	os.Exit(0)
}

func main() {
	// set PRNG seed
	rand.Seed(time.Now().UnixNano())

	// create doji directory
	err := os.Mkdir(pwd, os.ModeDir|0755)
	if err != nil && !os.IsExist(err) {
		log.Fatalln(err)
	}
	ioutil.WriteFile(path.Join(pwd, "index.html"), []byte("nothing to see"), 0644)

	// setup logging
	log.SetFlags(log.Ltime | log.LUTC | log.Lshortfile)
	if os.Getenv("DEBUG") == "" {
		file := path.Join(pwd, "log")
		l, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalln(err)
		}
		log.SetOutput(l)
		defer l.Close()
	}

	// load templates
	tmpl = tmpl.Funcs(template.FuncMap{"rndname": rndName})
	for _, file := range []string{
		"room.gtl", "help.gtl", "info.gtl", "script.js",
	} {
		asset, err := Asset(path.Join("static", file))
		if err != nil {
			log.Fatalln(err)
		}
		tmpl = template.Must(tmpl.New(file).Parse(string(asset)))
	}

	// setup sets
	err = initSets()
	if err != nil {
		log.Fatalln(err)
	}

	// process signals
	go sigCatch()

	// configure HTTP
	http.Handle("/d/", http.StripPrefix("/d/", http.FileServer(http.Dir(pwd))))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})
	http.HandleFunc("/", room)

	// start HTTP Server
	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8080"
	}
	log.Println("Listening on", listen)
	log.Fatal(http.ListenAndServe(listen, nil))
}
