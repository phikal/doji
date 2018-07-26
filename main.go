package main

//go:generate go-bindata -o templ.go foyer.gtl parlor.gtl

import (
	"bufio"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	ws "github.com/gorilla/websocket"
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

var (
	// id -> parlor map and global lock
	parlors = make(map[string]*Parlor)
	lock    = &sync.Mutex{}

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
)

// Msg stores websocket messages, which are sent from and to the client
type Msg struct {
	Type string      `json:"type"`
	Msg  string      `json:"msg,omitempty"`
	Val  float64     `json:"val,omitempty"`
	From string      `json:"from,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// User represents, and contains, a websocket connection
type User struct {
	key  string
	conn *ws.Conn
	msg  chan<- Msg
}

// a parlor (video-room, session) stores the current state of the room,
// as well as the users (in a map) and further internal data
type Parlor struct {
	Paused   bool             // is current video paused
	Progress float64          // seconds into current video
	Users    map[string]*User // users (id -> User) in this parlor
	Videos   []string         // videos available to be selected
	Watching string           // video currently being watched
	Key      string           // parlors key pointing to this Parlor
	lock     *sync.Mutex      // lock for .Users map
	updated  time.Time        // last status update recived
	notif    chan<- *User     // send a status update to this user
	format   string           // ytdl-format (-f) to use
}

// generate a random string
func rndName() string {
	chars := "aeiouxyz"
	for {
		name := make([]byte, 3)
		for i := 0; i < 3; i++ {
			name[i] = chars[rand.Int()%len(chars)]
		}

		if _, ok := parlors[string(name)]; !ok {
			return string(name)
		}
	}
}

func (u *User) talker() {
	c := make(chan Msg)
	u.msg = c
	for {
		u.conn.WriteJSON(<-c)
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

func (p *Parlor) processCommands(u *User, fields []string) bool {
	if len(fields) < 1 {
		return false
	}

	switch fields[0] {
	case "/format":
		if len(fields) > 1 {
			p.format = fields[1]
		}
		u.msg <- Msg{
			Type: msgReqst,
			Msg:  fmt.Sprintf("set format to %q", p.format),
		}
	}
	return strings.HasPrefix(fields[0], "/")
}

// continuously listen on a connection and process incoming messages, as
// well as start a goroutine to coordinate outgoing messages
func (p *Parlor) processConn(conn *ws.Conn, user string) {
	u := &User{conn: conn}

	p.lock.Lock()
	p.Users[user] = u
	p.lock.Unlock()

	go u.talker()
	defer p.cleanUp(u)

	var msg Msg
	for {
		err := conn.ReadJSON(&msg)
		if err != nil {
			return
		}

		// interpret message
		switch msg.Type {
		case msgTalk:
			fields := strings.Fields(msg.Msg)
			if p.processCommands(u, fields) {
				continue
			}
		case msgPlay:
			p.Paused = false
		case msgPause:
			p.Paused = true
			fallthrough
		case msgSeek:
			p.Progress = msg.Val
		case msgSelect:
			p.Watching = msg.Msg
			p.Paused = true
			p.Progress = 0
		case msgReqst:
			go p.getVideo(msg.Msg)
		}
		p.updated = time.Now()

		// re-process message
		switch msg.Type {
		case msgTalk, msgPause, msgPlay, msgSeek, msgSelect:
			msg.From = user
			for _, u := range p.Users {
				u.msg <- msg
			}
		case msgStatus:
			for _, u := range p.Users {
				p.notif <- u
			}
		}
	}
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

	for _, u := range p.Users {
		p.notif <- u
	}
}

// notify all clients about download updates
func (p *Parlor) wall(msg string) {
	for _, c := range p.Users {
		c.msg <- Msg{
			Type: msgReqst,
			Msg:  msg,
		}
	}
}

// use youtube-dl to download a video
func (p *Parlor) getVideo(url string) {
	cmd := exec.Command("youtube-dl", []string{
		"--verbose",
		"--newline",
		"--no-part",
		"--restrict-filenames",
		"-f",
		p.format,
		"-o",
		path.Join(p.Key, "%(title)s-%(id)s.%(ext)s"),
		url,
	}...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.wall(fmt.Sprintf("Error: %s", err))
		return
	}
	out := bufio.NewScanner(stdout)

	if err = cmd.Start(); err != nil {
		p.wall(fmt.Sprintf("Error: %s", err))
		return
	}

	p.wall(fmt.Sprintf("Started downloading %s...", url))

	var progress float64 = 0.0
	for out.Scan() {
		var perc float64
		fmt.Sscanf(out.Text(), "[download]  %f%%", &perc)
		if perc > progress+10 {
			p.wall(fmt.Sprintf("Downloaded %.2f%%", perc))
			progress = perc
		}

		if perc > 10 && perc < 20 {
			p.loadVideos()
		}
	}

	if err = cmd.Wait(); err != nil {
		p.wall(fmt.Sprintf("Error: %s", err))
		return
	}

	p.loadVideos()
	p.wall(fmt.Sprintf("Finished downloading %s", url))
}

// wait for requests to send users the current status
func (p *Parlor) statusMonitor() {
	var msg Msg
	msg.Type = msgStatus
	notif := make(chan *User, 4)
	p.notif = notif

	for {
		user := <-notif

		progress := p.Progress
		if !p.Paused {
			progress += time.Since(p.updated).Seconds()
		}
		msg.Data = map[string]interface{}{
			"vids":     p.Videos,
			"users":    p.Users,
			"paused":   p.Paused,
			"playing":  p.Watching,
			"progress": progress,
		}

		if user.msg != nil {
			user.msg <- msg
		} else {
			notif <- user
			time.Sleep(time.Millisecond * 50)
		}

	}
}

// http handler for creating parlors and websockets
func parlor(w http.ResponseWriter, r *http.Request) {
	// create or join existing parlor
	if r.Method == http.MethodPost {
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
		return
	}

	// get requested parlor-id
	var name string
	if _, err := fmt.Sscanf(r.URL.Path, "/%s", &name); err != nil {
		http.Error(w, "no name specified", http.StatusBadRequest)
		return // shouldn't happen
	}

	// create websocket
	if strings.HasSuffix(name, "/socket") {
		name = strings.TrimSuffix(name, "/socket")

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
	} else {
		if err := tmpl.ExecuteTemplate(w, "parlor", parlors[name]); err != nil {
			log.Fatal(err)
			return
		}
	}
}

// handle user signals
func sigHandler(c <-chan os.Signal, temp string) {
	for {
		switch <-c {
		case syscall.SIGUSR1:
			for _, p := range parlors {
				p.loadVideos()
			}
		case syscall.SIGINT, syscall.SIGQUIT:
			os.RemoveAll(temp)
			os.Exit(0)
		}
	}
}

func main() {
	// check for youtube-dl
	if _, err := exec.LookPath("youtube-dl"); err != nil {
		log.Fatalln(os.Stderr, "failed to find youtube-dl in PATH")
	}

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
