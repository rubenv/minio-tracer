package main

import "log"

func main() {
	err := do()
	if err != nil {
		log.Fatal(err)
	}
}

func do() error {
	return nil
}
