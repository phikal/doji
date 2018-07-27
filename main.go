package main

//go:generate go-bindata -o templ.go style.css foyer.gtl parlor.gtl

import (
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
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
)

var style []byte

func main() {
	// set PRNG seed
	rand.Seed(time.Now().UTC().UnixNano())

	if asset, err := Asset("parlor.gtl"); err != nil {
		log.Fatalln(err)
	} else {
		tmpl = template.Must(tmpl.New("parlor").Parse(string(asset)))
	}

	if asset, err := Asset("foyer.gtl"); err != nil {
		log.Fatalln(err)
	} else {
		tmpl = template.Must(tmpl.New("foyer").Parse(string(asset)))
	}

	if asset, err := Asset("style.css"); err != nil {
		log.Fatalln(err)
	} else {
		style = asset
	}

	// create directory structure
	pwd, err := ioutil.TempDir("", "doji-")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.Chdir(pwd); err != nil {
		log.Fatalln(err)
	}
	ioutil.WriteFile(path.Join(pwd, "index.html"), []byte("nothing to see"), 0644)

	// process signals
	schan := make(chan os.Signal)
	signal.Notify(schan, os.Interrupt, os.Kill)
	go sigHandler(schan, pwd)

	// configure HTTP
	http.Handle("/d/", http.StripPrefix("/d/", http.FileServer(http.Dir(pwd))))
	http.HandleFunc("/style.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", mime.TypeByExtension(".css")+"; charset=utf-8")
		if _, err := w.Write(style); err != nil {
			log.Fatalln(err)
		}
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.Method == http.MethodPost {
			parlor(w, r)
		} else if err := tmpl.ExecuteTemplate(w, "foyer", nil); err != nil {
			log.Fatal(err)
		}
	})

	// start HTTP Server
	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = ":8080"
	}
	log.Fatal(http.ListenAndServe(listen, nil))
}
