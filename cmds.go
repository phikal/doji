package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"regexp"
	"strings"
)

var cmdRe = regexp.MustCompile(`^/(\w+)(?:\s+(.+)\s*)?`)

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
	switch {
	case parts[2] != "":
		arg = parts[1]
		fallthrough
	case parts[1] != "":
		log.Printf("%s/%s: executed %s (%d)", p.Key, u.key, msg, len(parts))
	default:
		return false
	}

	log.Println("Unlocking")
	p.Lock()
	defer p.Unlock()
	log.Println("Unlocked")
	switch parts[0] {
	case "unmute":
		u.msg <- Msg{
			Type: msgVol,
			Val:  1,
		}
	case "mute":
		u.msg <- Msg{
			Type: msgVol,
			Val:  0,
		}
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
	// case "delete":
	// 	if arg != "" {
	// 		p.del(arg, rq)
	// 	}
	// case "u", "update":
	// 	p.loadVideos()
	case "progress":
		rq("Playing %q at %f", p.Watching.Name, p.Progress())
	case "next", "n":
		p.Watching = p.Queue[0]
		p.Queue = p.Queue[1:]
		p.update(0, true)
		p.notifyAll()

		for _, u := range p.Users {
			u.msg <- Msg{Type: msgPlay}
		}
	case "f", "format":
		if arg != "" {
			p.format = arg
		}
		rq("Set format to %q", p.format)
	case "op", "oper", "operator":
		if arg == "" {
			rq(fmt.Sprintf("is operator: %t", u.oper))
			break
		}

		if u.oper {
			for _, user := range p.Users {
				if user.Name == arg {
					user.oper = true
					p.notifyAll()
					break
				}
			}

			rq("op: No such user")
		} else {
			rq("you can't op someone if you aren't an op")
		}
	default:
		rq("No such command %q", parts[0])
	}

	return strings.HasPrefix(arg, "/")
}
