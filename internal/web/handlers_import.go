package web

import (
	"net/http"

	"github.com/simon/dewey/internal/csvimport"
	"github.com/simon/dewey/internal/store"
)

type importData struct {
	Error              string
	FileName           string
	RowErrors          []csvimport.RowError
	Summary            *store.ImportSummary
	TeacherPlaceholder string
}

func newImportData() importData {
	return importData{TeacherPlaceholder: store.ImportTeacherPlaceholder}
}

func (s *Server) handleImportForm(w http.ResponseWriter, r *http.Request) {
	render(w, "import", newImportData())
}

func (s *Server) handleImportUpload(w http.ResponseWriter, r *http.Request) {
	d := newImportData()

	if err := r.ParseMultipartForm(csvimport.MaxBytes); err != nil {
		d.Error = "Datei konnte nicht gelesen werden. Bitte eine CSV-Datei auswählen (höchstens 4 MB)."
		s.renderImportError(w, d)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		d.Error = "Bitte eine CSV-Datei auswählen."
		s.renderImportError(w, d)
		return
	}
	defer file.Close()
	d.FileName = header.Filename

	res, err := csvimport.Parse(file)
	if err != nil {
		d.Error = err.Error()
		s.renderImportError(w, d)
		return
	}
	d.RowErrors = res.Errors

	if len(res.Students) == 0 {
		if len(res.Errors) == 0 {
			d.Error = "Die Datei enthält keine Schüler."
		}
		s.renderImportError(w, d)
		return
	}

	toImport := make([]store.ImportStudent, len(res.Students))
	for i, st := range res.Students {
		toImport[i] = store.ImportStudent{
			Name:       st.Name,
			ClassYear:  st.ClassYear,
			ClassLabel: st.ClassLabel,
		}
	}

	summary, err := s.store.ImportStudents(r.Context(), toImport)
	if err != nil {
		_, msg := s.friendlyError(r, err)
		d.Error = "Import fehlgeschlagen: " + msg
		s.renderImportError(w, d)
		return
	}

	s.logger.Info("csv import",
		"file", header.Filename,
		"classes_created", summary.ClassesCreated,
		"students_created", summary.StudentsCreated,
		"students_skipped", summary.StudentsSkipped,
		"rows_rejected", len(res.Errors))

	d.Summary = &summary
	render(w, "import", d)
}

func (s *Server) renderImportError(w http.ResponseWriter, d importData) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, "import", d)
}
