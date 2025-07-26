package main

import (
	"fmt"
	"flag"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
)

var (
	infoLog = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime)
	errLog = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)
	httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns: 200,
			MaxIdleConnsPerHost: 200,
		},
	}
)

func handleRequest(rw http.ResponseWriter, r *http.Request) {
	requestUrl := r.URL.String()
	infoLog.Printf("%s %s from %s", r.Method, requestUrl, r.RemoteAddr)

	req, err := http.NewRequest(r.Method, requestUrl, r.Body)
	if err != nil {
		errLog.Printf("Failed to create outgoing request: %v", err)
		http.Error(rw, "Error creating request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header = r.Header

	res, err := httpClient.Do(req)
	if err != nil {
		errLog.Printf("Failed to send outgoing request: %v", err)
		http.Error(rw, "Error sending request: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer res.Body.Close()

	maps.Copy(rw.Header(), res.Header)
	rw.WriteHeader(res.StatusCode)
	io.Copy(rw, res.Body)
}

func main() {
	port := flag.String("port", "8080", "Port for the proxy server to listen on")
	quiet := flag.Bool("quiet", false, "Disable info logs")
	flag.Parse()

	if *quiet {
		infoLog.SetOutput(io.Discard)
	}
	infoLog.Println("Starting server on port", *port)

	addr := fmt.Sprintf(":%s", *port)
	http.HandleFunc("/", handleRequest)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		errLog.Fatalf("Could not start http server: %v", err)
	}
}
