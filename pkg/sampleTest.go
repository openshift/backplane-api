package pkg

import (
	"fmt"
	"net/http"
)

func sampleFunc() {
	fmt.Println("Hello world")
}

// A HandlerFunc function
func RootHandler(w http.ResponseWriter, req *http.Request) {
    fmt.Fprintln(w, "Hello, world!")
}
