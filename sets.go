package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"sync"
)

type Set []string

var (
	sets     map[string]Set
	setNames []string
	setLock  sync.Mutex
)

func (p *Room) toggleSet(setName string) error {
	set, ok := sets[setName]
	if !ok {
		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	// p.Lock()
	defer p.loadVideos()
	// defer p.Unlock()

	_, loaded := p.Sets[setName]
	log.Printf("Toggled set (%v) %q in %q", loaded, setName, p.Key)

	if !loaded { // load
		for _, v := range set {
			err = os.Symlink(v, path.Join(wd, p.Key, path.Base(v)))
			if err != nil {
				log.Println(err)
				return err
			}
		}
		p.Sets[setName] = len(set)
	} else { // unload
		for _, v := range set {
			file := path.Join(wd, p.Key, path.Base(v))
			if _, err = os.Stat(file); err == nil && file != p.Watching {
				err = os.Remove(file)
				if err != nil {
					log.Println(err)
					return err
				}
			}
		}
		delete(p.Sets, setName)
	}
	return nil
}

func loadSets(setDir string) error {
	setLock.Lock()
	defer setLock.Unlock()
	sets = make(map[string]Set)
	setNames = nil

	log.Println("Loading sets from ", setDir)
	setDirs, err := ioutil.ReadDir(setDir)
	if err != nil {
		return err
	}

	for _, set := range setDirs {
		name := set.Name()
		sets[name] = nil

		s, err := ioutil.ReadDir(path.Join(setDir, name))
		if err != nil {
			return err
		}

		for _, vid := range s {
			sets[name] = append(sets[name], path.Join(setDir, name, vid.Name()))
		}

		setNames = append(setNames, name)
		log.Printf("Added set %q with %d items", name, len(sets[name]))
	}

	return nil
}
