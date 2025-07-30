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

	"github.com/wilblik/proxyonthego/internal/log"
	"gopkg.in/yaml.v3"
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

type ServiceData struct {
	Path      string
	Instances []*url.URL
	nextIndex uint64
}

func (s *ServiceData) nextInstance() *url.URL {
	idx := atomic.AddUint64(&s.nextIndex, 1) % uint64(len(s.Instances))
	return s.Instances[idx]
}

func getServiceData(config *Config) []*ServiceData {
	var services []*ServiceData
	for _, sc := range config.Services {
		var instances  []*url.URL
		for _, instanceURL := range sc.Instances {
			instances = append(instances, instanceURL.URL)
		}
		if len(instances) == 0 {
			log.LogFatalf("No instance URL configured for service path: %s", sc.Path)
		}
		services = append(services, &ServiceData {
			Path: sc.Path,
			Instances : instances,
		})
		log.LogInfo("Configured service for path '%s' with %d instances", sc.Path, len(instances))
	}
	return services
}

func createHandler(services []*ServiceData) http.Handler {
	mux := http.NewServeMux()
	for _, service := range services {
		proxy := createReverseProxy(service)
		handler := http.Handler(proxy)
		// Don't strp prefix in case of "/" path because we end up with empty path and return 404
		if service.Path != "/" {
			handler = http.StripPrefix(service.Path, handler)
			// Handles trailing "/" to prevent ServeMux from returning "Moved Permanently"
			mux.Handle(service.Path+"/", handler)
		}
		mux.Handle(service.Path, handler)
	}
	return mux
}

func createReverseProxy(service *ServiceData) *httputil.ReverseProxy  {
	proxy := &httputil.ReverseProxy{}
	proxy.Director = func(req *http.Request) {
		target := service.nextInstance()
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
	}
	return proxy
}

func main() {
	configPath := flag.String("config", getDefaultConfigPath(), "Path to the configuration file")
	quiet := flag.Bool("quiet", false, "Disable info logs")
	flag.Parse()

	if *quiet {
		log.DisableInfo()
	}

	config := parseConfig(configPath)
	serviceData := getServiceData(&config)
	handler := createHandler(serviceData)
	port := fmt.Sprintf(":%s", config.Port)

	if config.TLS != nil && config.TLS.CertFile != "" && config.TLS.KeyFile != "" {
		log.LogInfo("Starting https reverse proxy on port %s", config.Port)
		log.LogFatal(http.ListenAndServeTLS(port, config.TLS.CertFile, config.TLS.KeyFile, handler))
	} else {
		log.LogInfo("Starting http reverse proxy on port %s", config.Port)
		log.LogFatal(http.ListenAndServe(port, handler))
	}
}
