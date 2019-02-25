package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

// Set is a list of videos that is stored on the server, and can be
// loaded or unloaded dynamically by users in a room
type Set struct {
	id      string
	content []*Video
}

var (
	sets    map[string]*Set
	setLock sync.Mutex
)

func (p *Room) toggleSet(name string) {
	set, ok := sets[name]
	if !ok {
		return
	}

	i := sort.Search(len(p.Sets),
		func(i int) bool { return p.Sets[i] == set })

	p.Lock()
	if i < len(p.Sets) && p.Sets[i] == set {
		p.Sets = append(p.Sets[:i], p.Sets[i+1:]...)
	} else {
		p.Sets = append(p.Sets, set)
	}
	p.Unlock()
}

func initSets() error {
	setLock.Lock()
	defer setLock.Unlock()
	sets = make(map[string]*Set)

	setDir := os.Getenv("SETDIR")
	if setDir != "" {
		var err error
		setDir, err = filepath.Abs(setDir)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		log.Println("no sets loaded")
		return nil
	}

	log.Println("Loading sets from ", setDir)
	dirs, err := ioutil.ReadDir(setDir)
	if err != nil {
		return err
	}

	var id int
	for _, set := range dirs {
		name := set.Name()

		s, err := ioutil.ReadDir(path.Join(setDir, name))
		if err != nil {
			return err
		}

		var S Set = Set{id: strconv.Itoa(id)}
		for _, vid := range s {
			S.content = append(S.content, &Video{
				Path:  path.Join("/d/", name),
				Set:   name,
				Name:  vid.Name(),
				Ready: true,
			})
		}
		sets[name] = &S
		id += 1
	}

	return nil
}
