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
		log.Fatalln("failed to find youtube-dl in PATH")
	}
}

// use youtube-dl to download a video
func (p *Room) getVideo(url string) {
	wall := func(msg string) {
		for _, c := range p.Users {
			c.msg <- Msg{
				Type: msgReqst,
				Msg:  msg,
			}
		}
	}

	<-p.reqs

	r := Request{Url: url}
	p.Lock()
	p.Requests = append(p.Requests, &r)
	p.Unlock()

	log.Printf("Downloading %s as %s", url, p.format)

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

		_, err := fmt.Sscanf(out.Text(), "[download]  %f%%", &perc)
		if err != nil {
			continue
		}

		if r.Progress == 0 && perc == 100 {
			continue
		}

		r.Progress = perc / 100

		p.notifyAll()
	}

	if err = r.cmd.Wait(); err != nil {
		wall(fmt.Sprintf("Error: %s", err))
		return
	}

	p.Lock()
	for i, R := range p.Requests {
		if &r == R {
			p.Requests = append(p.Requests[:i], p.Requests[i+1:]...)
			break
		}
	}
	p.Unlock()

	p.loadVideos()
	p.reqs <- true
}
