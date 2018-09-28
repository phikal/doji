package main

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"path"
)

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

	r := Video{Url: url}
	p.Lock()
	var set = p.Sets[""]
	*set = append(*set, &r)
	p.Sets[""] = set
	p.Unlock()

	log.Printf("Downloading %s as %s", url, p.format)

	r.cmd = exec.Command("youtube-dl", []string{
		"--verbose",
		"--newline",
		"--no-part",
		"--restrict-filenames",
		"--prefer-free-formats",
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
		_, err := fmt.Sscanf(out.Text(), "[download] Destination: %c", &r.Name)
		if err != nil {
			continue
		}

		var perc float64
		_, err = fmt.Sscanf(out.Text(), "[download]  %f%%", &perc)
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

	r.Ready = true
}
