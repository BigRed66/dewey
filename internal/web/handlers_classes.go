package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/simon/dewey/internal/store"
)

type classesData struct {
	Classes   []classRow
	EditClass *classRow
	Error     string
}

type classRow struct {
	ID      int64
	Year    int
	Label   string
	Teacher string
}

func (s *Server) handleClassesList(w http.ResponseWriter, r *http.Request) {
	classes, err := s.store.ListClasses(r.Context())
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	rows := make([]classRow, len(classes))
	for i, c := range classes {
		rows[i] = classRow{ID: c.ID, Year: c.Year, Label: c.Label, Teacher: c.Teacher}
	}
	d := classesData{Classes: rows}
	if editID, _ := strconv.ParseInt(r.URL.Query().Get("edit"), 10, 64); editID > 0 {
		if c, err := s.store.GetClass(r.Context(), editID); err == nil {
			d.EditClass = &classRow{ID: c.ID, Year: c.Year, Label: c.Label, Teacher: c.Teacher}
		}
	}
	render(w, "classes", d)
}

func (s *Server) handleClassesUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	year, err := strconv.Atoi(r.FormValue("year"))
	if err != nil || year < 1 || year > 4 {
		s.classesWithError(w, r, "Jahrgang muss zwischen 1 und 4 liegen.")
		return
	}
	label := r.FormValue("label")
	teacher := r.FormValue("teacher")
	if label == "" || teacher == "" {
		s.classesWithError(w, r, "Gruppe und Lehrkraft sind erforderlich.")
		return
	}
	if err := s.store.UpdateClass(r.Context(), id, year, label, teacher); err != nil {
		_, msg := s.friendlyError(r, err)
		s.classesWithError(w, r, "Klasse konnte nicht aktualisiert werden: "+msg)
		return
	}
	http.Redirect(w, r, "/classes", http.StatusSeeOther)
}

func (s *Server) handleClassesDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.store.DeleteClass(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrClassOnLoan) {
			s.classesWithError(w, r, "Klasse konnte nicht gelöscht werden: Schüler haben noch ausgeliehene Bücher.")
			return
		}
		_, msg := s.friendlyError(r, err)
		s.classesWithError(w, r, "Klasse konnte nicht gelöscht werden: "+msg)
		return
	}
	http.Redirect(w, r, "/classes", http.StatusSeeOther)
}

func (s *Server) handleClassesCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	year, err := strconv.Atoi(r.FormValue("year"))
	if err != nil || year < 1 || year > 4 {
		s.classesWithError(w, r, "Jahrgang muss zwischen 1 und 4 liegen.")
		return
	}
	label := r.FormValue("label")
	teacher := r.FormValue("teacher")
	if label == "" || teacher == "" {
		s.classesWithError(w, r, "Gruppe und Lehrkraft sind erforderlich.")
		return
	}

	if _, err := s.store.CreateClass(r.Context(), year, label, teacher); err != nil {
		_, msg := s.friendlyError(r, err)
		s.classesWithError(w, r, "Klasse konnte nicht erstellt werden: "+msg)
		return
	}
	http.Redirect(w, r, "/classes", http.StatusSeeOther)
}

func (s *Server) classesWithError(w http.ResponseWriter, r *http.Request, msg string) {
	classes, _ := s.store.ListClasses(r.Context())
	rows := make([]classRow, len(classes))
	for i, c := range classes {
		rows[i] = classRow{ID: c.ID, Year: c.Year, Label: c.Label, Teacher: c.Teacher}
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, "classes", classesData{Classes: rows, Error: msg})
}
