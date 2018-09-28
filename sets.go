package main

import (
	"io/ioutil"
	"log"
	"path"
	"sync"
)

// Set is a list of videos that is stored on the server, and can be
// loaded or unloaded dynamically by users in a room
type Set []*Video

// func (s Set) Len() int           { return len(s) }
// func (s Set) Less(i, j int) bool { return s[i].Name < s[j].Name }
// func (s Set) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

var (
	sets    map[string]*Set
	setLock sync.Mutex
)

func (p *Room) toggleSet(name string) {
	set, ok := sets[name]
	if !ok {
		return
	}

	_, loaded := p.Sets[name]
	p.Lock()
	if loaded {
		delete(p.Sets, name)
	} else {
		p.Sets[name] = set
	}
	p.Unlock()
}

func initSets() error {
	setLock.Lock()
	defer setLock.Unlock()
	sets = make(map[string]*Set)

	log.Println("Loading sets from ", setDir)
	dirs, err := ioutil.ReadDir(setDir)
	if err != nil {
		return err
	}

	for _, set := range dirs {
		name := set.Name()

		s, err := ioutil.ReadDir(path.Join(setDir, name))
		if err != nil {
			return err
		}

		var S Set
		for _, vid := range s {
			S = append(S, &Video{
				Path:  path.Join("/d/", name),
				Set:   name,
				Name:  vid.Name(),
				Ready: true,
			})
		}
		sets[name] = &S
	}

	return nil
}
