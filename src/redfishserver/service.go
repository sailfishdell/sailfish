package redfishserver

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path"
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
    backendFuncMap template.FuncMap
    getViewData func(string, string, map[string]string)map[string]interface{}
    mapURLToTemplate func(string) (string, map[string]string)
}

type Config struct {
    BackendFuncMap template.FuncMap
    GetViewData func(string, string, map[string]string)map[string]interface{}
    MapURLToTemplate func(string) (string, map[string]string)
}

// right now macos doesn't support plugins, so main executable configures this
// and passes it in. Later this will use plugin loading infrastructure
func NewService(logger Logger, templatesDir string, backendConfig Config) RedfishService {
    var err error
	rh := &redfishService{root: templatesDir, backendFuncMap: backendConfig.BackendFuncMap, getViewData: backendConfig.GetViewData, mapURLToTemplate: backendConfig.MapURLToTemplate}

	rh.loadConfig = func(exitOnErr bool) {
		templatePath := path.Join(templatesDir, "*.json")
		logger.Log("msg", "Loading config from path", "path", templatePath)
		tempTemplate := template.New("the template")
        tempTemplate.Funcs(rh.backendFuncMap)
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

	return rh
}

// ServiceMiddleware is a chainable behavior modifier for RedfishService.
type ServiceMiddleware func(RedfishService) RedfishService

func (rh *redfishService) GetRedfish(ctx context.Context, r *http.Request) ([]byte, error) {
	templateName, args := rh.mapURLToTemplate(r.URL.Path)

	logger := RequestLogger(ctx)
	logger.Log("templatename", templateName)

	rh.templateLock.RLock()
	defer rh.templateLock.RUnlock()
	buf := new(bytes.Buffer)
    viewData := rh.getViewData(r.URL.Path, templateName, args)
	rh.templates.ExecuteTemplate(buf, templateName, viewData)
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
