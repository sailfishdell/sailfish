package server

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"text/template"
)

type RedfishService interface {
	GetRedfish(ctx context.Context, r *http.Request) ([]byte, error)
	PutRedfish(ctx context.Context, r *http.Request) ([]byte, error)
	PostRedfish(ctx context.Context, r *http.Request) ([]byte, error)
	PatchRedfish(ctx context.Context, r *http.Request) ([]byte, error)
	DeleteRedfish(ctx context.Context, r *http.Request) ([]byte, error)
}

type redfishService struct {
	root         string
	templateLock sync.RWMutex
	templates    *template.Template
	loadConfig   func(bool)
	pageModel    map[string]interface{}
}

func NewService(logger Logger, templatesDir string, pluginName string) RedfishService {
	rh := &redfishService{root: templatesDir}

	loadConfig := func(exitOnErr bool) {
		templatePath := path.Join(templatesDir, "*.json")
		logger.Log("msg", "Loading config from path", "path", templatePath)
		tempTemplate, err := template.New("the template").ParseGlob(templatePath)
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

	loadConfig(false)

	return rh
}

func getViewData(rh *redfishService, templateName string) interface{} {
    return nil
}

// ServiceMiddleware is a chainable behavior modifier for RedfishService.
type ServiceMiddleware func(RedfishService) RedfishService

func (rh *redfishService) GetRedfish(ctx context.Context, r *http.Request) ([]byte, error) {
	templateName := r.URL.Path + "/index.json"
	templateName = strings.Replace(templateName, "//", "/", -1)
	templateName = strings.Replace(templateName, "/", "_", -1)
	if strings.HasPrefix(templateName, "_") {
		templateName = templateName[1:]
	}
	if strings.HasPrefix(templateName, "redfish_v1_") {
		templateName = templateName[len("redfish_v1_"):]
	}

	logger := RequestLogger(ctx)
	logger.Log("templatename", templateName)

	rh.templateLock.RLock()
	defer rh.templateLock.RUnlock()
	buf := new(bytes.Buffer)
	rh.templates.ExecuteTemplate(buf, templateName, getViewData(rh, templateName))
	return buf.Bytes(), nil
}

func (rh *redfishService) PutRedfish(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *redfishService) PostRedfish(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *redfishService) PatchRedfish(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *redfishService) DeleteRedfish(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}
