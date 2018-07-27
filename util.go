package main

import (
	"log"
	"math/rand"
	"os"
)

const consonants = "skwvyxz"
const vouls = "aeiou"

// generate a random string
func rndName() string {
	for {
		name := make([]byte, 4)
		var dict string = consonants
		for i := 0; i < 4; i++ {
			name[i] = dict[rand.Int()%len(dict)]

			if dict == vouls {
				dict = consonants
			} else {
				dict = vouls
			}
		}

		if _, ok := parlors[string(name)]; !ok {
			return string(name)
		}
	}
}

// handle user signals
func sigHandler(c <-chan os.Signal, temp string) {
	<-c
	err := os.RemoveAll(temp)
	if err != nil {
		log.Panicln(err)
	}
	os.Exit(0)
}
