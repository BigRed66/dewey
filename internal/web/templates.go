package web

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

var pages map[string]*template.Template

var funcs = template.FuncMap{
	"sub": func(a, b int) int { return a - b },
}

func init() {
	pages = make(map[string]*template.Template)
	for _, name := range []string{"classes", "students", "books", "books_new", "loans", "loans_checkout", "book_detail", "student_detail", "settings"} {
		pages[name] = template.Must(template.New("").Funcs(funcs).ParseFS(templateFS,
			"templates/layout.html",
			"templates/"+name+".html",
		))
	}
}

func render(w http.ResponseWriter, page string, data any) {
	t, ok := pages[page]
	if !ok {
		http.Error(w, "unknown page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
