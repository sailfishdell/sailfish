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
	RedfishGet(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishPut(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishPost(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishPatch(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishDelete(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishHead(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishOptions(ctx context.Context, r *http.Request) ([]byte, error)
}

type redfishService struct {
	root             string
	templateLock     sync.RWMutex
	templates        *template.Template
	loadConfig       func(bool)
	backendFuncMap   template.FuncMap
	getViewData      func(*http.Request, string, map[string]string) map[string]interface{}
	mapURLToTemplate func(*http.Request) (string, map[string]string, error)
}

type Config struct {
	BackendFuncMap   template.FuncMap
	GetViewData      func(*http.Request, string, map[string]string) map[string]interface{}
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

type templateParams struct {
	Args     map[string]string
	ViewData map[string]interface{}
}

func checkHeaders(ctx context.Context, r *http.Request) (err error) {
    // TODO: check Content-Type (for things with request body only)
    // TODO: check OData-MaxVersion "Indicates the maximum version of OData
    //                              that an odata-aware client understands"
    // TODO: check OData-Version "Services shall reject requests which specify
    //                              an unsupported OData version. If a service
    //                              encounters a version that it does not
    //                              support, the service should reject the
    //                              request with status code [412]
    //                              (#status-412). If client does not specify
    //                              an Odata-Version header, the client is
    //                              outside the boundaries of this
    //                              specification."
    return
}

func (rh *redfishService) RedfishGet(ctx context.Context, r *http.Request) ([]byte, error) {
	logger := RequestLogger(ctx)

    err := checkHeaders(ctx, r)
    if err != nil {
        return nil, err
    }

	templateName, args, err := rh.mapURLToTemplate(r)
	if err != nil {
		logger.Log("error", "Error getting mapping for URL", "url", r.URL.Path)
		return nil, err
	}

	rh.templateLock.RLock()
	defer rh.templateLock.RUnlock()

	buf := new(bytes.Buffer)
	viewData := rh.getViewData(r, templateName, args)

	rh.templates.ExecuteTemplate(buf, templateName, templateParams{ViewData: viewData, Args: args})

    // TODO: need a mechanism to return headers that the encoder will add
    //       sketch: return a struct containing output plus funcs to set headers
    // TODO: ETag -
    // TODO: X-Auth-Token - (SHALL)
    // TODO: Retry-After
    // TODO: WWW-Authenticate - (SHALL)
    // TODO: Server
    // TODO: Link
	return buf.Bytes(), nil
}

func (rh *redfishService) RedfishPut(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *redfishService) RedfishPost(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *redfishService) RedfishPatch(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *redfishService) RedfishDelete(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *redfishService) RedfishHead(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *redfishService) RedfishOptions(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}
