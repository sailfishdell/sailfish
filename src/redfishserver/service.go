package redfishserver

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"sync"
	"text/template"
)

type Service interface {
	TemplatedRedfishGet(ctx context.Context, templateName, url string, args map[string]string) (interface{}, error)
	RawJSONRedfishGet(ctx context.Context, url string, args map[string]string) (interface{}, error)
}

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

type Config struct {
	BackendFuncMap      template.FuncMap
	GetViewData         func(context.Context, string, string, map[string]string) (map[string]interface{}, error)
	GetJSONOutput       func(context.Context, string, map[string]string) (interface{}, error)

	// private fields
	root         string
	templateLock sync.RWMutex
	templates    *template.Template
	loadConfig   func(bool)
}

var (
	ErrNotFound = errors.New("not found")
)

// right now macos doesn't support plugins, so main executable configures this
// and passes it in. Later this will use plugin loading infrastructure
func NewService(logger Logger, templatesDir string, rh Config) Service {
	var err error

	rh.root = templatesDir
	rh.loadConfig = func(exitOnErr bool) {
		templatePath := path.Join(templatesDir, "*json")
		logger.Log("msg", "Loading templates from path", "path", templatePath)
		tempTemplate := template.New("the template")
		tempTemplate.Funcs(rh.BackendFuncMap)
		tempTemplate, err = tempTemplate.ParseGlob(templatePath)

		if err != nil {
			logger.Log("msg", "Fatal error parsing template", "err", err)
			if exitOnErr {
				os.Exit(1)
			}
		}
		rh.templateLock.Lock()
		rh.templates = tempTemplate
		rh.templateLock.Unlock()
	}

	rh.loadConfig(false)

	return &rh
}

type templateParams struct {
	Args     map[string]string
	ViewData map[string]interface{}
}

func (rh *Config) TemplatedRedfishGet(ctx context.Context, templateName, url string, args map[string]string) (interface{}, error) {
	logger := RequestLogger(ctx)
    logger.Log("msg", "HELLO WORLD")

	buf := new(bytes.Buffer)
	viewData, err := rh.GetViewData(ctx, templateName, url, args)
    if err != nil {
        logger.Log("msg", "ERROR EARLY RETURN")
        return nil, err
    }

    rh.templateLock.RLock()
    rh.templates.ExecuteTemplate(buf, templateName, templateParams{ViewData: viewData, Args: args})
    rh.templateLock.RUnlock()

	output := buf.Bytes()
	return output, err
}


func (rh *Config) RawJSONRedfishGet(ctx context.Context, url string, args map[string]string) (interface{}, error) {
	logger := RequestLogger(ctx)
    logger.Log("msg", "HELLO WORLD: rawjson")

	return rh.GetJSONOutput(ctx, url, args)
}
