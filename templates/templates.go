package templates

import (
	"embed"
	"html/template"
)

//go:embed *.html
var FS embed.FS

func Must() *template.Template {
	return template.Must(template.ParseFS(FS, "*.html"))
}
