package main

import (
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"sync"
	"time"

	"github.com/wilblik/proxyonthego/internal/breaker"
	"github.com/wilblik/proxyonthego/internal/log"
)

// TODO Provide a way to configure
const (
	BreakerFailuresThreshold = 5
	BreakerResetTimeout = 30 * time.Second
)

var (
	httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns: 200,
			MaxIdleConnsPerHost: 200,
		},
	}
	breakers = make(map[string]*breaker.CircuitBreaker)
	breakersMutex = &sync.Mutex{}
)

func getBreaker(host string) *breaker.CircuitBreaker {
	breakersMutex.Lock()
	defer breakersMutex.Unlock()
	if cb, exists := breakers[host]; exists {
		return cb
	}
	newCb := breaker.New(BreakerFailuresThreshold, BreakerResetTimeout)
	breakers[host] = newCb
	log.LogInfo("Created new circuit breaker for host: %s", host)
	return newCb
}

func handleRequest(rw http.ResponseWriter, r *http.Request) {
	requestUrl := r.URL.String()
	log.LogInfo("%s %s from %s", r.Method, requestUrl, r.RemoteAddr)

	breaker := getBreaker(r.URL.Host)
	if !breaker.Ready() {
		log.LogErr("Circuit breaker for %s is open", requestUrl)
		http.Error(rw, "Service is not available", http.StatusServiceUnavailable)
		return
	}

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
		breaker.RecordFailure()
		return
	}
	defer res.Body.Close()

	if res.StatusCode >= 500 {
		breaker.RecordFailure()
	} else {
		breaker.RecordSuccess()
	}

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
