package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
)

var cmdRe = regexp.MustCompile(`^/(\w+)(?:\s+(\w+))?`)

func (p *Room) processCommands(u *User, msg string) bool {
	log.Printf("User %q in %q wrote %q\n", u.key, p.Key, msg)

	rq := func(format string, a ...interface{}) {
		u.msg <- Msg{
			Type: msgReqst,
			Msg:  fmt.Sprintf(format, a...),
		}
	}

	URL, err := url.ParseRequestURI(msg)
	if err == nil && URL.IsAbs() {
		rq("<strong>%s</strong> requested to download <code>%s</code>", u.key, msg)
		go p.getVideo(msg)
		return true
	}

	var arg string
	parts := cmdRe.FindStringSubmatch(msg)
	switch len(parts) {
	case 2:
		arg = parts[1]
		fallthrough
	case 1:
		log.Printf("%s/%s: executed %s (%d)", p.Key, u.key, msg, len(parts))
	case 0:
		return false
	}

	log.Println("Unlocking")
	p.Lock()
	log.Println("Unlocked")
	defer p.Unlock()
	switch parts[0] {
	case "stats", "stat":
		fi, err := ioutil.ReadDir(p.Key)
		if err != nil {
			rq("Failed to read files: %s", err)
			break
		}

		var sum int64
		for _, f := range fi {
			sum += f.Size()
		}

		rq("All files require %.2f MiB", float64(sum)/(1<<20))
	case "delete":
		if arg == "" {
			break
		}

		r, err := regexp.Compile(arg)
		if err != nil {
			break
		}

		for i, vid := range p.Videos {
			if r.MatchString(vid) {
				err := os.Remove(path.Join(p.Key, vid))
				if err != nil {
					rq("Failed to delete %s: %s", vid, err)
					break
				} else {
					rq("Deleted %s", vid)
				}

				p.Videos = append(p.Videos[:i], p.Videos[i+1:]...)
				p.notifyAll()
			}
		}

		p.loadVideos()
	case "u", "update":
		p.loadVideos()
	case "next", "n":
		p.notifyAll()

		for _, u := range p.Users {
			u.msg <- Msg{Type: msgPlay}
		}
	case "f", "format":
		if arg != "" {
			p.format = arg
		}
		rq("Set format to %q", p.format)
	default:
		rq("No such command %q", parts[0])
	}

	return strings.HasPrefix(arg, "/")
}
