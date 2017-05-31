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
	loadConfig   (bool)
}

func NewService(rootPath string, logger Logger) RedfishService {
	rh := &redfishService{root: rootPath}

	loadConfig := func(exitOnErr bool) {
		templatePath := path.Join(rootPath, "*.json")
		logger.Log("path", templatePath)
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

// ServiceMiddleware is a chainable behavior modifier for RedfishService.
type ServiceMiddleware func(RedfishService) RedfishService

func (rh *redfishService) GetRedfish(ctx context.Context, r *http.Request) ([]byte, error) {
	templateName := r.URL.Path + "/index.json"
	templateName = strings.Replace(templateName, "/", "_", -1)
	if strings.HasPrefix(templateName, "_") {
		templateName = templateName[1:]
	}
	if strings.HasPrefix(templateName, "redfish_v1_") {
		templateName = templateName[len("redfish_v1_"):]
	}

	logger := RequestLogger(ctx)
	logger.Log("Template_Start", templateName)
	defer logger.Log("Template_Done", templateName)

	rh.templateLock.RLock()
	defer rh.templateLock.RUnlock()
	buf := new(bytes.Buffer)
	rh.templates.ExecuteTemplate(buf, templateName, nil)
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
