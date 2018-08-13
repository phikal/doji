package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
)

const MAX_PROCS = 5

type Request struct {
	Url      string  `json:"url"`
	Progress float64 `json:"progress"`
	cmd      *exec.Cmd
}

// check for youtube-dl
func init() {
	if _, err := exec.LookPath("youtube-dl"); err != nil {
		log.Fatalln(os.Stderr, "failed to find youtube-dl in PATH")
	}
}

// use youtube-dl to download a video
func (p *Parlor) getVideo(url string) {
	wall := func(msg string) {
		for _, c := range p.Users {
			c.msg <- Msg{
				Type: msgReqst,
				Msg:  msg,
			}
		}
	}

	if len(p.Dlds) >= MAX_PROCS {
		<-p.reqs
	}

	r := Request{Url: url}
	p.lock.Lock()
	p.Dlds = append(p.Dlds, r)
	p.lock.Unlock()

	r.cmd = exec.Command("youtube-dl", []string{
		"--verbose",
		"--newline",
		"--no-part",
		"--restrict-filenames",
		"-f", p.format,
		"-o", path.Join(p.Key, "%(title)s-%(id)s.%(ext)s"),
		url,
	}...)

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		wall(fmt.Sprintf("Error: %s", err))
		return
	}
	out := bufio.NewScanner(stdout)

	if err = r.cmd.Start(); err != nil {
		wall(fmt.Sprintf("Error: %s", err))
		return
	}

	for out.Scan() {
		var perc float64
		fmt.Sscanf(out.Text(), "[download]  %f%%", &perc)

		if r.Progress == 0 && perc == 100 {
			continue
		}

		if perc > r.Progress+10 {
			r.Progress = perc
		}

		for _, u := range p.Users {
			p.notif <- u
		}
	}

	if err = r.cmd.Wait(); err != nil {
		wall(fmt.Sprintf("Error: %s", err))
		return
	}

	p.lock.Lock()
	for i, R := range p.Dlds {
		if R == r {
			p.Dlds = append(p.Dlds[:i], p.Dlds[i+1:]...)
			break
		}
	}
	p.lock.Unlock()
	p.reqs <- true
}
