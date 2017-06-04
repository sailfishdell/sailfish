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

const HTTP_HEADER_SERVER = "go-redfish/0.1"

type RedfishService interface {
	RedfishGet(ctx context.Context, r *http.Request) (interface{}, error)
	RedfishPut(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishPost(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishPatch(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishDelete(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishHead(ctx context.Context, r *http.Request) ([]byte, error)
	RedfishOptions(ctx context.Context, r *http.Request) ([]byte, error)
}

type serviceBackendConfig struct {
	root             string
	templateLock     sync.RWMutex
	templates        *template.Template
	loadConfig       func(bool)
	backendFuncMap   template.FuncMap
	getViewData      func(context.Context, *http.Request, string, map[string]string) map[string]interface{}
	mapURLToTemplate func(*http.Request) (string, map[string]string, error)
    postProcessTemplate func(context.Context, *http.Request, []byte, string, map[string]string)  map[string]string
}

type Config struct {
    // module-private data
    InstanceData      interface{}
	MapURLToTemplate  func(*http.Request) (string, map[string]string, error)
	BackendFuncMap    template.FuncMap
	GetViewData       func(context.Context, *http.Request, string, map[string]string) map[string]interface{}
    PostProcessTemplate func(context.Context, *http.Request, []byte, string, map[string]string)  map[string]string
}

// right now macos doesn't support plugins, so main executable configures this
// and passes it in. Later this will use plugin loading infrastructure
func NewService(logger Logger, templatesDir string, backendConfig Config) RedfishService {
	var err error
	rh := &serviceBackendConfig{root: templatesDir, backendFuncMap: backendConfig.BackendFuncMap, getViewData: backendConfig.GetViewData, mapURLToTemplate: backendConfig.MapURLToTemplate, postProcessTemplate: backendConfig.PostProcessTemplate}

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

func (rh *serviceBackendConfig) RedfishGet(ctx context.Context, r *http.Request) (interface{}, error) {
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

	buf := new(bytes.Buffer)

	viewData := rh.getViewData(ctx, r, templateName, args)

	if len(templateName) > 0 {
		rh.templateLock.RLock()
		rh.templates.ExecuteTemplate(buf, templateName, templateParams{ViewData: viewData, Args: args})
		rh.templateLock.RUnlock()
	}

    output := buf.Bytes()

    var headers map[string]string
    headers = make(map[string]string)
    headers["Server"] = HTTP_HEADER_SERVER
    if rh.postProcessTemplate != nil {
	    headers = rh.postProcessTemplate(ctx, r, output, templateName, args)
    }

    // need to evaluate all of the headers we'll need
	// TODO: ETag -
	// TODO: X-Auth-Token - (SHALL)
	// TODO: Retry-After
	// TODO: WWW-Authenticate - (SHALL) Used for 401 (Unauthorized) requests to indicate the authentication schemes that are usable
	// TODO: Server
	// TODO: Link

    return func(ctx context.Context, w http.ResponseWriter) error {
            for k,v := range headers {
                w.Header().Set(k,v)
            }

            //w.Header().Set("Access-Control-Allow-Origin", "*")
            //w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            //w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

	        _, err := w.Write(output)
            return err
    }, nil
}

func (rh *serviceBackendConfig) RedfishPut(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *serviceBackendConfig) RedfishPost(ctx context.Context, r *http.Request) ([]byte, error) {
	// TODO: HTTP HEADER: Location - (CONDITIONAL SHALL) for POST to return where the object was created
	return []byte(""), nil
}

func (rh *serviceBackendConfig) RedfishPatch(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *serviceBackendConfig) RedfishDelete(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *serviceBackendConfig) RedfishHead(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}

func (rh *serviceBackendConfig) RedfishOptions(ctx context.Context, r *http.Request) ([]byte, error) {
	return []byte(""), nil
}
