package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sync"
)

type Set []string

var (
	setDir  string
	sets    map[string]Set
	setLock sync.Mutex
)

func (p *Room) toggleSet(setName string) (loaded bool, err error) {
	set, ok := sets[setName]
	if !ok {
		err = fmt.Errorf("No such set %q", setName)
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		return
	}

	p.Lock()
	defer p.Unlock()
	defer p.loadVideos()

	if _, loaded = p.Sets[setName]; ok { // unload
		for _, v := range set {
			err = os.Symlink(v, path.Join(wd, p.Key, path.Base(v)))
			if err != nil {
				return
			}
		}
		delete(p.Sets, setName)
		return
	} else {
		for _, v := range set {
			err = os.Remove(path.Join(wd, p.Key, path.Base(v)))
			if err != nil {
				return
			}
		}
		p.Sets[setName] = len(set)
		return
	}
}

func loadSets() {
	if setDir == "" {
		return
	}

	setLock.Lock()
	defer setLock.Unlock()
	sets = make(map[string]Set)

	setDirs, err := ioutil.ReadDir(setDir)
	if err != nil {
		log.Println(err)
		return
	}

	for _, set := range setDirs {
		name := set.Name()
		sets[name] = nil

		s, err := ioutil.ReadDir(path.Join(setDir, name))
		if err != nil {
			log.Println(err)
			return
		}

		for _, vid := range s {
			sets[name] = append(sets[name],
				path.Join(setDir, name, vid.Name()))
		}
	}
}
