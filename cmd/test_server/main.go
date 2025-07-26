// A simple HTTP server to act as the destination for our proxy tests.

package main

import (
	"fmt"
	"log"
	"net/http"
	"flag"
)

func main() {
	port := flag.String("port", "8080", "Port for the http server to listen on")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	addr := fmt.Sprintf(":%s", *port)
	log.Fatal(http.ListenAndServe(addr, nil))
}
