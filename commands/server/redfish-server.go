package main

import (
	"flag"
	"github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	"github.com/gorilla/mux"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/superchalupa/go-redfish/src/server"
	"net/http"
	"os"
    "io/ioutil"
    "github.com/go-yaml/yaml"
)

type AppConfig struct {
    Listen string  `yaml: listen`
    TemplatesDir string `yaml: templatesDir`
    BackendPluginName string `yaml: backendPluginName`
}

func loadConfig(filename string) (AppConfig, error) {
    bytes, err := ioutil.ReadFile(filename)
    if err != nil {
        return AppConfig{}, err
    }

    var config AppConfig
    err = yaml.Unmarshal(bytes, &config)
    if err != nil {
        return AppConfig{}, err
    }

    return config, nil
}

func main() {
	var (
		configFile   = flag.String("config", "app.yaml", "Application configuration file")
		listen   = flag.String("listen", ":8080", "HTTP listen address")
		templatesDir = flag.String("templates", "serve", "base path from which to serve redfish data templates")
		pluginName = flag.String("backendPlugin", "null", "name of the backend plugin")
	)
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(os.Stderr)

    appConfig, _:= loadConfig(*configFile)
    if len(*listen) > 0 { appConfig.Listen = *listen }
    if len(*pluginName) > 0 { appConfig.BackendPluginName = *pluginName }
    if len(*templatesDir) > 0 { appConfig.TemplatesDir = *templatesDir }

	logger = log.With(logger, "listen", *listen, "caller", log.DefaultCaller)

	var svc server.RedfishService
	svc = server.NewService(logger, appConfig.TemplatesDir, appConfig.BackendPluginName)
	svc = server.NewLoggingService(logger, svc)

	fieldKeys := []string{"method", "URL"}
	svc = server.NewInstrumentingService(
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

	r := mux.NewRouter()
	server.NewRedfishHandler(svc, r)

	http.Handle("/", r)
	http.Handle("/metrics", promhttp.Handler())

	logger.Log("msg", "HTTP", "addr", appConfig.Listen)
	logger.Log("err", http.ListenAndServe(appConfig.Listen, nil))
}
