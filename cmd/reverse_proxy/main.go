package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/wilblik/proxyonthego/internal/breaker"
	"github.com/wilblik/proxyonthego/internal/log"
	"gopkg.in/yaml.v3"
)

// TODO Provide a way to configure
const (
	BreakerFailuresThreshold = 5
	BreakerResetTimeout = 30 * time.Second
)

type Config struct {
	Port     string          `yaml:"port"`
	TLS      *TLSConfig      `yaml:"tls"`
	Services []ServiceConfig `yaml:"services"`
}

type TLSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type ServiceConfig struct {
	Path      string    `yaml:"path"`
	Instances []YAMLURL `yaml:"instances"`
}

type YAMLURL struct {
    *url.URL
}

func (yamlUrl *YAMLURL) UnmarshalYAML(unmarshal func(any) error) error {
    var s string
    err := unmarshal(&s)
    if err != nil {
        return err
    }
    url, err := url.Parse(s)
	if err != nil {
		return err
	}
	if url.Scheme != "http" && url.Scheme != "https" {
		return fmt.Errorf("%s is not a valid http or https URL", url)
	}
	if url.Host == "" {
		return fmt.Errorf("%s is not a valid URL. Host Is missing", url)
	}
    yamlUrl.URL = url
    return nil
}

func getDefaultConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		log.LogFatalf("Could not determine executable path: %v", err)
	}
	return filepath.Join(filepath.Dir(exePath), "config.yaml")
}

func parseConfig(configPath *string) Config {
	configContent, err := os.ReadFile(*configPath)
	if err != nil {
		log.LogFatalf("Could not read config file '%s': %v", *configPath, err)
	}
	var config Config
	err = yaml.Unmarshal(configContent, &config)
	if err != nil {
		log.LogFatalf("Could not parse config file '%s': %v", *configPath, err)
	}
	return config
}

type ServiceManager struct {
	Path      string
	Instances []*ServiceInstance
	nextIndex uint64
}

type ServiceInstance struct {
	Url     *url.URL
	Proxy   *httputil.ReverseProxy
	Breaker *breaker.CircuitBreaker
}

func (s *ServiceManager) nextHealthyInstance() *ServiceInstance {
	for i := 0; i < len(s.Instances); i++ {
		idx := atomic.AddUint64(&s.nextIndex, 1) % uint64(len(s.Instances))
		instance := s.Instances[idx]
		if instance.Breaker.Ready() {
			return instance
		}
	}
	return nil
}

func getServiceManagers(config *Config) []*ServiceManager {
	var serviceMgrs []*ServiceManager
	for _, serviceConf := range config.Services {
		var instances  []*ServiceInstance
		for _, instanceURL := range serviceConf.Instances {
			breaker := breaker.New(BreakerFailuresThreshold, BreakerResetTimeout)
			proxy := createReverseProxy(instanceURL.URL, breaker)
			serviceInstance := &ServiceInstance{Url: instanceURL.URL, Proxy: proxy, Breaker: breaker}
			instances = append(instances, serviceInstance)
		}
		if len(instances) == 0 {
			log.LogFatalf("No instance URL configured for service path: %s", serviceConf.Path)
		}
		serviceMgrs = append(serviceMgrs, &ServiceManager{Path: serviceConf.Path,Instances: instances})
		log.LogInfo("Configured service for path '%s' with %d instances", serviceConf.Path, len(instances))
	}
	return serviceMgrs
}

func createReverseProxy(target *url.URL, breaker *breaker.CircuitBreaker) *httputil.ReverseProxy  {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = func(res *http.Response) error {
		if res.StatusCode >= 500 {
			breaker.RecordFailure()
		} else {
			breaker.RecordSuccess()
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		breaker.RecordFailure()
		log.LogErr("Error calling '%s': %v", req.URL, err)
		http.Error(w, "Service is not available", http.StatusServiceUnavailable)
	}
	return proxy
}

func createHandler(serviceMgrs []*ServiceManager) http.Handler {
	mux := http.NewServeMux()
	for _, serviceMgr := range serviceMgrs {
		handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			instance := serviceMgr.nextHealthyInstance()
			if instance == nil {
				log.LogErr("All backends for service: %s are down", serviceMgr.Path)
				http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
				return
			}
			log.LogInfo("Forwarding request for service '%s' to %s", serviceMgr.Path, instance.Url)
			instance.Proxy.ServeHTTP(w, r)
		})
		handler := http.Handler(handlerFunc)
		// Don't strp prefix in case of "/" path because then we end up with empty path and return 404
		if serviceMgr.Path != "/" {
			handler = http.StripPrefix(serviceMgr.Path, handler)
			// Handles trailing "/" to prevent ServeMux from returning "Moved Permanently"
			mux.Handle(serviceMgr.Path+"/", handler)
		}
		mux.Handle(serviceMgr.Path, handler)
	}
	return mux
}

func main() {
	configPath := flag.String("config", getDefaultConfigPath(), "Path to the configuration file")
	quiet := flag.Bool("quiet", false, "Disable info logs")
	flag.Parse()

	if *quiet {
		log.DisableInfo()
	}

	config := parseConfig(configPath)
	serviceMgrs := getServiceManagers(&config)
	handler := createHandler(serviceMgrs)
	port := fmt.Sprintf(":%s", config.Port)

	if config.TLS != nil && config.TLS.CertFile != "" && config.TLS.KeyFile != "" {
		log.LogInfo("Starting https reverse proxy on port %s", config.Port)
		log.LogFatal(http.ListenAndServeTLS(port, config.TLS.CertFile, config.TLS.KeyFile, handler))
	} else {
		log.LogInfo("Starting http reverse proxy on port %s", config.Port)
		log.LogFatal(http.ListenAndServe(port, handler))
	}
}
