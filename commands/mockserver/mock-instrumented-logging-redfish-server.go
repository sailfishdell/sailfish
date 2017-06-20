package main

import (
	"flag"
	"github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	"github.com/go-yaml/yaml"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"strings"

	"github.com/superchalupa/go-redfish/src/redfishserver"
)

type appConfig struct {
	Listen       string `yaml:"listen"`
	TemplatesDir string `yaml:"templatesDir"`
}

func loadConfig(filename string) (appConfig, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return appConfig{}, err
	}

	var config appConfig
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return appConfig{}, err
	}

	return config, nil
}

func main() {
	var (
		configFile   = flag.String("config", "app.yaml", "Application configuration file")
		listen       = flag.String("listen", ":8080", "HTTP listen address")
		templatesDir = flag.String("templates", "serve", "base path from which to serve redfish data templates")
	)
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	cfg, _ := loadConfig(*configFile)
	if len(*listen) > 0 {
		cfg.Listen = *listen
	}
	if len(*templatesDir) > 0 {
		cfg.TemplatesDir = *templatesDir
	}

	logger = log.With(logger, "listen", cfg.Listen, "caller", log.DefaultCaller)

	var svc redfishserver.Service
	svc = redfishserver.NewService(logger, cfg.TemplatesDir)
	svc = redfishserver.NewLoggingService(logger, svc)

	fieldKeys := []string{"method", "URL"}
	svc = redfishserver.NewInstrumentingService(
		kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "redfish",
			Subsystem: "redfish_service",
			Name:      "request_count",
			Help:      "Number of requests received.",
		}, fieldKeys),
		kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "redfish",
			Subsystem: "redfish_service",
			Name:      "request_latency_microseconds",
			Help:      "Total duration of requests in microseconds.",
		}, fieldKeys),
		svc,
	)

	r := redfishserver.NewRedfishHandler(svc, logger)

	done := svc.Startup()
	defer close(done)

	http.Handle("/", r)
	http.Handle("/metrics", promhttp.Handler())

	var listener net.Listener
	var err error
	switch {
	case strings.HasPrefix(cfg.Listen, "fcgi:") && strings.Contains(strings.TrimPrefix(cfg.Listen, "fcgi:"), ":"):
		addr := strings.TrimPrefix(cfg.Listen, "fcgi:")
		logger.Log("msg", "FCGI mode activated with tcp listener: "+addr)
		listener, err = net.Listen("tcp", addr)

	case strings.HasPrefix(cfg.Listen, "fcgi:") && strings.Contains(strings.TrimPrefix(cfg.Listen, "fcgi:"), "/"):
		path := strings.TrimPrefix(cfg.Listen, "fcgi:")
		logger.Log("msg", "FCGI mode activated with unix socket listener: "+path)
		listener, err = net.Listen("unix", path)
		defer os.Remove(path)

	case strings.HasPrefix(cfg.Listen, "fcgi:"):
		logger.Log("msg", "FCGI mode activated with stdin/stdout listener")
		listener = nil

	default:
		logger.Log("msg", "HTTP", "addr", cfg.Listen)
		logger.Log("err", http.ListenAndServe(cfg.Listen, nil))
	}

	if strings.HasPrefix(cfg.Listen, "fcgi:") {
		if err != nil {
			logger.Log("fatal", "Could not open listening connection", "err", err)
			return
		}
		if listener != nil {
			defer listener.Close()
		}
		logger.Log("err", fcgi.Serve(listener, r))
	}
}
