package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/zuiwuchang/cb/cmd"
)

func main() {
	b := sha1.Sum([]byte("cerberus us is an idea"))
	fmt.Println(hex.EncodeToString(b[:]))
	os.Exit(1)
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	e := cmd.Execute()
	if e != nil {
		log.Fatalln(e)
	}
}
