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

type Service interface {
	RedfishGet(ctx context.Context, r *http.Request) (interface{}, error)
}


type Config struct {
	MapURLToTemplate  func(*http.Request) (string, map[string]string, error)
	BackendFuncMap    template.FuncMap
	GetViewData       func(context.Context, *http.Request, string, map[string]string) map[string]interface{}
    PostProcessTemplate func(context.Context, *http.Request, []byte, string, map[string]string)  map[string]string

    // private fields
	root             string
	templateLock     sync.RWMutex
	templates        *template.Template
	loadConfig       func(bool)
}

// right now macos doesn't support plugins, so main executable configures this
// and passes it in. Later this will use plugin loading infrastructure
func NewService(logger Logger, templatesDir string, rh Config) Service {
	var err error

    rh.root = templatesDir
	rh.loadConfig = func(exitOnErr bool) {
		templatePath := path.Join(templatesDir, "*.json")
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

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

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

func (rh *Config) RedfishGet(ctx context.Context, r *http.Request) (interface{}, error) {
	logger := RequestLogger(ctx)

	err := checkHeaders(ctx, r)
	if err != nil {
		return nil, err
	}

	templateName, args, err := rh.MapURLToTemplate(r)
	if err != nil {
		logger.Log("error", "Error getting mapping for URL", "url", r.URL.Path)
		return nil, err
	}

	buf := new(bytes.Buffer)

	viewData := rh.GetViewData(ctx, r, templateName, args)

	if len(templateName) > 0 {
		rh.templateLock.RLock()
		rh.templates.ExecuteTemplate(buf, templateName, templateParams{ViewData: viewData, Args: args})
		rh.templateLock.RUnlock()
	}

    output := buf.Bytes()

    var headers map[string]string
    headers = make(map[string]string)
    headers["Server"] = HTTP_HEADER_SERVER
    if rh.PostProcessTemplate != nil {
	    headers = rh.PostProcessTemplate(ctx, r, output, templateName, args)
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
