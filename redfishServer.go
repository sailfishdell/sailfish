package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"text/template"
    "strings"

	"github.com/gorilla/mux"
)

type redfishHandler struct {
	root string
    thisTmpl *template.Template
}

func (rh *redfishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
    ctx := r.Context()
    fmt.Fprintf(os.Stdout, "context: %s\n", ctx)
    fmt.Fprintf(os.Stdout, "vars: %s\n", vars)
	fmt.Fprintf(os.Stdout, "Serving URL: %s\n", r.URL.Path)

    templateName := r.URL.Path + "/index.json"
    templateName = strings.Replace(templateName, "/", "_", -1)
    if strings.HasPrefix(templateName, "_") {
        templateName = templateName[1:]
    }

    fmt.Fprintf(os.Stdout, "Executing template: %s\n", templateName)

	rh.thisTmpl.ExecuteTemplate(w, templateName, nil)

	fmt.Fprintf(os.Stdout, "done executing\n")
}


func main() {
	var rootpath = flag.String("root", "serve", "help message")
	flag.Parse()

    templatePath :=  path.Join(*rootpath, "*.json")
    fmt.Fprintf(os.Stdout, "the path is %s\n", templatePath )

    rh := &redfishHandler{root: *rootpath, thisTmpl: nil}

    loadConfig := func(canFail bool) {
        tempTemplate := template.Must(template.New("the template").ParseGlob(templatePath))
        rh.thisTmpl = tempTemplate
    }

    loadConfig(false)

	r := mux.NewRouter()
	r.PathPrefix("/redfish/v1/").Handler(http.StripPrefix("/redfish/v1/", rh))

	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}
