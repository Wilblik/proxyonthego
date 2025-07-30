// A simple HTTP server to act as the destination for our proxy tests.

package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/wilblik/proxyonthego/internal/log"
)

func main() {
	port := flag.String("port", "8080", "Port for the http server to listen on")
	certFile := flag.String("certFile", "", "Path to TLS certificate")
	keyFile := flag.String("keyFile", "", "Path to TLS private key")
	quiet := flag.Bool("quiet", false, "Disable info logs")
	flag.Parse()

	if *quiet {
		log.DisableInfo()
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reqDump, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.LogErr("Error dumping request: %v", err)
		} else {
			log.LogInfo("%s\n", string(reqDump))
		}
		fmt.Fprintln(w, "OK")
	})
	addr := fmt.Sprintf(":%s", *port)

	if *certFile != "" && *keyFile != "" {
		log.LogInfo("Starting https test server on port %s", *port)
		log.LogFatal(http.ListenAndServeTLS(addr, *certFile, *keyFile, nil))
	} else {
		log.LogInfo("Starting http test server on %s", addr)
		log.LogFatal(http.ListenAndServe(addr, nil))
	}
}
