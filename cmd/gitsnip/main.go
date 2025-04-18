package main

import (
	"flag"
	"fmt"
)

func main() {
	name := flag.String("name", "World", "a name to say hello to")
	flag.Parse()

	fmt.Printf("Hello, %s!\n", *name)
}
