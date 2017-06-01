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
    getViewData func(*http.Request, string, map[string]string)map[string]interface{}
    mapURLToTemplate func(*http.Request) (string, map[string]string, error)
}

type Config struct {
    BackendFuncMap template.FuncMap
    GetViewData func(*http.Request, string, map[string]string)map[string]interface{}
    MapURLToTemplate func(*http.Request) (string, map[string]string, error)
}

// right now macos doesn't support plugins, so main executable configures this
// and passes it in. Later this will use plugin loading infrastructure
func NewService(logger Logger, templatesDir string, backendConfig Config) RedfishService {
    var err error
	rh := &redfishService{root: templatesDir, backendFuncMap: backendConfig.BackendFuncMap, getViewData: backendConfig.GetViewData, mapURLToTemplate: backendConfig.MapURLToTemplate}

	rh.loadConfig = func(exitOnErr bool) {
		templatePath := path.Join(templatesDir, "*.json")
		logger.Log("msg", "Loading templates from path", "path", templatePath)
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
	logger := RequestLogger(ctx)

	templateName, args, err := rh.mapURLToTemplate(r)
    if err != nil {
        logger.Log("error", "Error getting mapping for URL", "url", r.URL.Path)
        return nil, err
    }

	logger.Log("templatename", templateName)
	logger.Log("defined_templates", rh.templates.DefinedTemplates())

	rh.templateLock.RLock()
	defer rh.templateLock.RUnlock()

	buf := new(bytes.Buffer)
    viewData := rh.getViewData(r, templateName, args)
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
