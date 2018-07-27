package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
)

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
		wall(fmt.Sprintf("Error: %s", err))
		return
	}
	out := bufio.NewScanner(stdout)

	if err = cmd.Start(); err != nil {
		wall(fmt.Sprintf("Error: %s", err))
		return
	}

	wall(fmt.Sprintf("Started downloading %s...", url))

	var progress float64 = 0.0
	for out.Scan() {
		var perc float64
		fmt.Sscanf(out.Text(), "[download]  %f%%", &perc)

		if progress == 0 && perc == 100 {
			continue
		}

		if perc > progress+10 {
			wall(fmt.Sprintf("Downloaded %.2f%%", perc))
			progress = perc
		}

		if perc > 10 && perc < 20 {
			p.loadVideos()
		}

		if perc == 100 {
			progress = 0
		}
	}

	if err = cmd.Wait(); err != nil {
		wall(fmt.Sprintf("Error: %s", err))
		return
	}

	wall(fmt.Sprintf("Finished downloading %s", url))
}
