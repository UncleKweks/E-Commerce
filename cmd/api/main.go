package main

import "fmt"

type user struct {
	Payload []byte
	Name    string
}

func main() {

	users := make(map[string]user, 100000)
	for i, u := range users {
		process(i, u)
	}
	fmt.Println("Hello, World!")

}

func process(i string, u user) {
	fmt.Printf("Processing user %s: %s\n", i, u.Name)
}
