package main

import (
	"os/exec"
)

type Video struct {
	Name     string  `json:"file"`
	Path     string  `json:"path"`
	Set      string  `json:"set,omitempty"`
	Url      string  `json:"url"`
	Progress float64 `json:"progress"`
	Ready    bool    `json:"ready"`
	cmd      *exec.Cmd
}

// func (p *Room) del(pat string, rq func(format string, a ...interface{})) error {
// 	r, err := regexp.Compile(pat)
// 	if err != nil {
// 		return err
// 	}

// 	for i, vid := range p.Videos {
// 		if r.MatchString(vid.Name) {
// 			err := os.Remove(path.Join(p.Key, vid.Name))
// 			if err != nil {
// 				rq("Failed to delete %s: %s", vid, err)
// 				break
// 			} else {
// 				rq("Deleted %s", vid)
// 			}

// 			p.Videos = append(p.Videos[:i], p.Videos[i+1:]...)
// 			p.notifyAll()
// 		}
// 	}

// 	p.loadVideos()
// 	return nil
// }
