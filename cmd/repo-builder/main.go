package main

import "log"

func main() {
	// Execute the root command
	if err := Execute(); err != nil {
		log.Fatal(err)
	}
}
