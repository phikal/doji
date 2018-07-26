package main

//go:generate go-bindata -o templ.go foyer.gtl parlor.gtl

import (
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path"
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
)

func main() {
	// set PRNG seed
	rand.Seed(time.Now().UTC().UnixNano())

	// parse templates
	for _, t := range []string{"parlor", "foyer"} {
		asset, err := Asset(t + ".gtl")
		if err != nil {
			log.Fatalln(err)
		}

		tmpl = template.Must(tmpl.New(t).Parse(string(asset)))
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
	signal.Notify(schan, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGUSR1)
	go sigHandler(schan, pwd)

	// configure HTTP
	http.Handle("/d/", http.StripPrefix("/d/", http.FileServer(http.Dir(pwd))))
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
