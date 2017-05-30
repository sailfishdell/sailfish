package main

import (
	"flag"
	"net/http"
	"os"
	"path"
	"text/template"
    "strings"
    "sync"
    "github.com/go-kit/kit/log"

    stdprometheus "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    kitprometheus "github.com/go-kit/kit/metrics/prometheus"

	"github.com/gorilla/mux"
)

type redfishHandler struct {
	root string
    templateLock sync.RWMutex
    templates *template.Template
    logger log.Logger
}

func (rh *redfishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // ctx := r.Context()
	// vars := mux.Vars(r)
    // rh.logger.Log( "context", ctx )
    // rh.logger.Log( "vars", vars )

    rh.logger.Log( "URL", r.URL.Path )

    templateName := r.URL.Path + "/index.json"
    templateName = strings.Replace(templateName, "/", "_", -1)
    if strings.HasPrefix(templateName, "_") {
        templateName = templateName[1:]
    }

    rh.logger.Log("Template_Start", templateName)
    defer rh.logger.Log("Template_Done", templateName)

    rh.templateLock.RLock()
    defer rh.templateLock.RUnlock()
	rh.templates.ExecuteTemplate(w, templateName, nil)
}

type loggingMW struct {
    logger log.Logger
}


func main() {
    var (
        listen = flag.String("listen", ":8080", "HTTP listen address")
	    rootpath = flag.String("root", "serve", "base path from which to serve redfish data templates")
    )
	flag.Parse()

    logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
    logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller, "listen", *listen)


    fieldKeys := []string{"method", "error"}
    requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
        Namespace: "redfish_group",
        Subsystem: "redfish_service",
        Name:      "request_count",
        Help:      "Number of requests received.",
    }, fieldKeys)
    requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
        Namespace: "redfish_group",
        Subsystem: "redfish_service",
        Name:      "request_latency_microseconds",
        Help:      "Total duration of requests in microseconds.",
    }, fieldKeys)
    countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
        Namespace: "redfish_group",
        Subsystem: "redfish_service",
        Name:      "count_result",
        Help:      "The result of each count method.",
    }, []string{})

    logger.Log("reqC", requestCount, "reqL", requestLatency, "countR", countResult)

    var rh *redfishHandler
    rh = &redfishHandler{root: *rootpath, templates: nil, logger: logger}

    loadConfig := func(exitOnErr bool) {
        templatePath :=  path.Join(*rootpath, "*.json")
        logger.Log("path", templatePath )
        tempTemplate, err := template.New("the template").ParseGlob(templatePath)
        if err != nil {
            logger.Log("Fatal error parsing template", err)
            if exitOnErr { os.Exit(1) }
        }
        rh.templateLock.Lock()
        rh.templates = tempTemplate
        rh.templateLock.Unlock()
    }

    loadConfig(false)
    s := make(chan os.Signal, 1)
    signal.Notify(s, syscall.SIGUSR2)
    go func() {
        for {
        <-s
        loadConfig(true)
        log.Println("Reloaded")
        }
    }()

	r := mux.NewRouter()
	r.PathPrefix("/redfish/v1/").Handler(http.StripPrefix("/redfish/v1/", rh))


    http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", r)

    logger.Log("msg", "starting HTTP", "addr", *listen)
    logger.Log("err", http.ListenAndServe(*listen, nil))
}
