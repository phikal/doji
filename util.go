package main

import (
	"math/rand"
	"os"
	"syscall"
)

const rchars = "aeiouxyz"

// generate a random string
func rndName() string {
	for {
		name := make([]byte, 4)
		for i := 0; i < 4; i++ {
			name[i] = rchars[rand.Int()%len(rchars)]
		}

		if _, ok := parlors[string(name)]; !ok {
			return string(name)
		}
	}
}

// handle user signals
func sigHandler(c <-chan os.Signal, temp string) {
	for {
		switch <-c {
		case syscall.SIGUSR1:
			for _, p := range parlors {
				p.loadVideos()
			}
		case syscall.SIGINT, syscall.SIGQUIT:
			os.RemoveAll(temp)
			os.Exit(0)
		}
	}
}
