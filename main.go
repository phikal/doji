package main

//go:generate go-bindata -o templ.go room.gtl

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
)

func main() {
	// set PRNG seed
	rand.Seed(time.Now().UnixNano())

	if asset, err := Asset("room.gtl"); err != nil {
		log.Fatalln(err)
	} else {
		tmpl = template.Must(template.New("room").
			Funcs(template.FuncMap{"rndname": rndName}).
			Parse(string(asset)))
	}

	// setup sets
	var err error
	setDir := os.Getenv("SETDIR")
	if setDir == "" {
		setDir, err = os.Getwd()
		if err != nil {
			log.Panicln(err)
		}
	}
	setDir, err = filepath.Abs(setDir)
	if err != nil {
		log.Fatalln(err)
	}
	if err := loadSets(setDir); err != nil {
		log.Fatalln(err)
	}

	// create directory structure
	pwd, err := ioutil.TempDir("", "doji-")
	if err != nil {
		log.Fatalln(err)
	}
	if err = os.Chdir(pwd); err != nil {
		log.Fatalln(err)
	}
	ioutil.WriteFile(path.Join(pwd, "index.html"), []byte("nothing to see"), 0644)

	if os.Getenv("DEBUG") == "" {
		l, err := os.Create(path.Join(pwd, "doji.log"))
		if err != nil {
			log.Println(err)
		} else if os.Getenv("DEBUG") == "" {
			log.SetOutput(l)
			defer l.Close()
		}
	}

	// process signals
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill, syscall.SIGUSR1)
	go func() {
		for {
			s := <-sig
			if s == syscall.SIGUSR1 {
				if err := loadSets(setDir); err != nil {
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
		os.Exit(0)
	}()

	// configure HTTP
	http.Handle("/d/", http.StripPrefix("/d/", http.FileServer(http.Dir(pwd))))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})
	http.HandleFunc("/", room)

	// start HTTP Server
	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = "localhost:8080"
	}
	log.Println("Listening on", listen)
	log.Fatal(http.ListenAndServe(listen, nil))
}
