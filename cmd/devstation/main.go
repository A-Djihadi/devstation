// Command devstation installe et configure un poste de dev Windows.
package main

import "fmt"

// version est remplacée à la compilation : -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	fmt.Println("devstation " + version)
}
