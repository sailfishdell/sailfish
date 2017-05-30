package main

import (
	"github.com/go-kit/kit/log"
	"os"
	"path"
	"sync"
	"text/template"
    "net/http"
    "strings"
    "bytes"
)

type RedfishService interface {
	GetRedfish(r *http.Request) ([]byte, error)
	PutRedfish(string) ([]byte, error)
	PostRedfish(string) ([]byte, error)
	PatchRedfish(string) ([]byte, error)
	DeleteRedfish(string) ([]byte, error)
}

type redfishService struct {
	root         string
	templateLock sync.RWMutex
	templates    *template.Template
	logger       log.Logger
	loadConfig   (bool)
}

func NewService(rootPath string, logger log.Logger) RedfishService {
	rh := &redfishService{root: rootPath, logger: logger}

	loadConfig := func(exitOnErr bool) {
		templatePath := path.Join(rootPath, "*.json")
		logger.Log("path", templatePath)
		tempTemplate, err := template.New("the template").ParseGlob(templatePath)
		if err != nil {
			logger.Log("Fatal error parsing template", err)
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

func (rh *redfishService) GetRedfish(r *http.Request) ([]byte, error) {
    templateName := r.URL.Path + "/index.json"
    templateName = strings.Replace(templateName, "/", "_", -1)
    if strings.HasPrefix(templateName, "_") {
        templateName = templateName[1:]
    }
    if strings.HasPrefix(templateName, "redfish_v1_") {
        templateName = templateName[len("redfish_v1_"):]
    }

    rh.logger.Log("Template_Start", templateName)
    defer rh.logger.Log("Template_Done", templateName)

    rh.templateLock.RLock()
    defer rh.templateLock.RUnlock()
    buf := new(bytes.Buffer)
    rh.templates.ExecuteTemplate(buf, templateName, nil)
    return buf.Bytes(), nil
}

func (rh *redfishService) PutRedfish(string) ([]byte, error) {
	return []byte(""), nil
}
func (rh *redfishService) PostRedfish(string) ([]byte, error) {
	return []byte(""), nil
}
func (rh *redfishService) PatchRedfish(string) ([]byte, error) {
	return []byte(""), nil
}
func (rh *redfishService) DeleteRedfish(string) ([]byte, error) {
	return []byte(""), nil
}
