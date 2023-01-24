package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift/backplane-api/pkg"
)

func main() {
    r := mux.NewRouter().StrictSlash(true)
    r.HandleFunc("/rootHandler", pkg.RootHandler)
 	fmt.Print(http.ListenAndServe(":8080", r)) 	
}
