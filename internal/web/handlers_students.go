package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/simon/dewey/internal/store"
)

type studentsData struct {
	Students    []store.Student
	Classes     []store.Class
	ClassID     int64
	EditStudent *store.Student
	Error       string
}

func (s *Server) handleStudentsList(w http.ResponseWriter, r *http.Request) {
	classes, err := s.store.ListClasses(r.Context())
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	var classID int64
	if raw := r.URL.Query().Get("class"); raw != "" {
		classID, _ = strconv.ParseInt(raw, 10, 64)
	}

	var students []store.Student
	if classID > 0 {
		students, err = s.store.ListStudentsByClass(r.Context(), classID)
	} else {
		students, err = s.store.ListStudents(r.Context())
	}
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	d := studentsData{Students: students, Classes: classes, ClassID: classID}
	if editID, _ := strconv.ParseInt(r.URL.Query().Get("edit"), 10, 64); editID > 0 {
		if st, err := s.store.GetStudent(r.Context(), editID); err == nil {
			d.EditStudent = &st
		}
	}
	render(w, "students", d)
}

type studentDetailData struct {
	Student      store.Student
	Loans        []loanRow
	ActiveCount  int
	TotalCount   int
	Initials     string
	Reservations []store.Reservation
	Books        []store.Book
	Error        string
	Success      string
}

func (s *Server) handleStudentDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	student, err := s.store.GetStudent(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	loans, err := s.store.AllLoansByStudent(r.Context(), id)
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	reservations, err := s.store.ActiveReservationsByStudent(r.Context(), id)
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	books, err := s.store.ListBooks(r.Context())
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	rows := make([]loanRow, len(loans))
	activeCount := 0
	for i, l := range loans {
		row := loanRow{Loan: l}
		if l.ReturnedAt == nil {
			activeCount++
			diff := time.Until(l.DueAt).Hours() / 24
			if diff < 0 {
				row.Overdue = true
				row.DaysOverdue = int(-diff)
			} else {
				row.DaysLeft = int(diff)
			}
			if reserved, err := s.store.ReservedByOtherStudent(r.Context(), l.Book.ID, id); err == nil {
				row.ReservedByOther = reserved
			}
		}
		rows[i] = row
	}

	render(w, "student_detail", studentDetailData{
		Student:      student,
		Loans:        rows,
		ActiveCount:  activeCount,
		TotalCount:   len(loans),
		Initials:     studentInitials(student.Name),
		Reservations: reservations,
		Books:        books,
		Error:        r.URL.Query().Get("error"),
		Success:      r.URL.Query().Get("success"),
	})
}

func studentInitials(name string) string {
	parts := strings.Fields(name)
	var b strings.Builder
	for _, p := range parts {
		r := []rune(p)
		if len(r) > 0 {
			b.WriteRune(unicode.ToUpper(r[0]))
		}
	}
	s := b.String()
	if len([]rune(s)) > 2 {
		return string([]rune(s)[:2])
	}
	return s
}

func (s *Server) handleStudentsUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	classID, err := strconv.ParseInt(r.FormValue("class_id"), 10, 64)
	if name == "" || err != nil || classID == 0 {
		s.studentsWithError(w, r, "Name und Klasse sind erforderlich.")
		return
	}
	if err := s.store.UpdateStudent(r.Context(), id, name, classID); err != nil {
		_, msg := s.friendlyError(r, err)
		s.studentsWithError(w, r, "Schüler konnte nicht aktualisiert werden: "+msg)
		return
	}
	http.Redirect(w, r, "/students", http.StatusSeeOther)
}

func (s *Server) handleStudentsDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.store.DeleteStudent(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrStudentOnLoan) {
			s.studentsWithError(w, r, "Schüler konnte nicht gelöscht werden: Es sind noch Bücher ausgeliehen.")
			return
		}
		_, msg := s.friendlyError(r, err)
		s.studentsWithError(w, r, "Schüler konnte nicht gelöscht werden: "+msg)
		return
	}
	http.Redirect(w, r, "/students", http.StatusSeeOther)
}

func (s *Server) handleStudentsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	classID, err := strconv.ParseInt(r.FormValue("class_id"), 10, 64)
	if name == "" || err != nil || classID == 0 {
		s.studentsWithError(w, r, "Name und Klasse sind erforderlich.")
		return
	}

	if _, err := s.store.CreateStudent(r.Context(), name, classID); err != nil {
		_, msg := s.friendlyError(r, err)
		s.studentsWithError(w, r, "Schüler konnte nicht erstellt werden: "+msg)
		return
	}
	http.Redirect(w, r, "/students", http.StatusSeeOther)
}

func (s *Server) studentsWithError(w http.ResponseWriter, r *http.Request, msg string) {
	classes, _ := s.store.ListClasses(r.Context())
	students, _ := s.store.ListStudents(r.Context())
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, "students", studentsData{
		Students: students,
		Classes:  classes,
		Error:    msg,
	})
}
