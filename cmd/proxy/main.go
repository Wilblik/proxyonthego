package main

import (
	"fmt"
	"flag"
	"io"
	"maps"
	"net/http"

	"github.com/wilblik/proxyonthego/internal/log"
)

var (
	httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns: 200,
			MaxIdleConnsPerHost: 200,
		},
	}
)

func handleRequest(rw http.ResponseWriter, r *http.Request) {
	requestUrl := r.URL.String()
	log.LogInfo("%s %s from %s", r.Method, requestUrl, r.RemoteAddr)

	req, err := http.NewRequest(r.Method, requestUrl, r.Body)
	if err != nil {
		log.LogErr("Failed to create outgoing request: %v", err)
		http.Error(rw, "Error creating request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header = r.Header

	res, err := httpClient.Do(req)
	if err != nil {
		log.LogErr("Failed to send outgoing request: %v", err)
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
	certFile := flag.String("certFile", "", "Path to TLS certificate")
	keyFile := flag.String("keyFile", "", "Path to TLS private key")
	quiet := flag.Bool("quiet", false, "Disable info logs")
	flag.Parse()

	if *quiet {
		log.DisableInfo()
	}

	http.HandleFunc("/", handleRequest)
	addr := fmt.Sprintf(":%s", *port)

	if *certFile != "" && *keyFile != "" {
		log.LogInfo("Starting https proxy server on port %s", *port)
		log.LogFatal(http.ListenAndServeTLS(addr, *certFile, *keyFile, nil))
	} else {
		log.LogInfo("Starting http proxy server on port %s", *port)
		log.LogFatal(http.ListenAndServe(addr, nil))
	}
}
