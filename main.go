package main

import (
	"log"

	"github.com/zuiwuchang/cb/cmd"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	e := cmd.Execute()
	if e != nil {
		log.Fatalln(e)
	}
}
