package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/simon/dewey/internal/store"
)

type loanRow struct {
	store.Loan
	DaysLeft        int
	DaysOverdue     int
	Overdue         bool
	Reservations    []store.Reservation
	ReservedByOther bool
}

type loansData struct {
	Loans           []loanRow
	Error           string
	Success         string
	Classes         []store.Class
	StudentsByClass []store.Student
}

type checkoutData struct {
	Classes         []store.Class
	StudentsByClass []store.Student
	StudentID       int64
	NFCTagID        string
	DefaultDueDate  string
	Error           string
}

func (s *Server) handleLoansList(w http.ResponseWriter, r *http.Request) {
	d, err := s.buildLoansData(r, r.URL.Query().Get("success"), r.URL.Query().Get("error"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	render(w, "loans", d)
}

func (s *Server) handleLoansCheckoutForm(w http.ResponseWriter, r *http.Request) {
	d, err := s.checkoutFormData(r)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	d.NFCTagID = r.URL.Query().Get("nfc_tag_id")
	d.DefaultDueDate = time.Now().AddDate(0, 0, store.LoanDays).Format("2006-01-02")
	render(w, "loans_checkout", d)
}

func (s *Server) handleLoansCheckout(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	studentID, err := strconv.ParseInt(r.FormValue("student_id"), 10, 64)
	nfcTagID := r.FormValue("nfc_tag_id")
	dueDateStr := r.FormValue("due_date")

	dueAt, parseErr := time.ParseInLocation("2006-01-02", dueDateStr, time.Local)
	if parseErr != nil {
		dueAt = time.Now().AddDate(0, 0, store.LoanDays)
	} else {
		dueAt = dueAt.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}

	if err != nil || studentID == 0 || nfcTagID == "" {
		d, _ := s.checkoutFormData(r)
		d.StudentID = studentID
		d.NFCTagID = nfcTagID
		d.DefaultDueDate = dueDateStr
		d.Error = "Schüler und Buch-Tag sind erforderlich."
		render(w, "loans_checkout", d)
		return
	}

	copy, err := s.store.GetCopyByNFCTag(r.Context(), nfcTagID)
	if err != nil {
		d, _ := s.checkoutFormData(r)
		d.StudentID = studentID
		d.NFCTagID = nfcTagID
		d.DefaultDueDate = dueDateStr
		if errors.Is(err, sql.ErrNoRows) {
			d.Error = "Kein Buch mit diesem Tag gefunden."
		} else {
			_, msg := s.friendlyError(r, err)
			d.Error = "Fehler: " + msg
		}
		render(w, "loans_checkout", d)
		return
	}

	onLoan, err := s.store.CopyOnLoan(r.Context(), copy.ID)
	if err != nil || onLoan {
		d, _ := s.checkoutFormData(r)
		d.StudentID = studentID
		d.NFCTagID = nfcTagID
		d.DefaultDueDate = dueDateStr
		d.Error = "Dieses Exemplar ist bereits ausgeliehen."
		render(w, "loans_checkout", d)
		return
	}

	if _, err := s.store.Checkout(r.Context(), studentID, copy.ID, dueAt); err != nil {
		_, msg := s.friendlyError(r, err)
		d, _ := s.checkoutFormData(r)
		d.StudentID = studentID
		d.NFCTagID = nfcTagID
		d.DefaultDueDate = dueDateStr
		d.Error = "Ausleihe fehlgeschlagen: " + msg
		render(w, "loans_checkout", d)
		return
	}

	http.Redirect(w, r, "/loans", http.StatusSeeOther)
}

func (s *Server) handleLoansReturn(w http.ResponseWriter, r *http.Request) {
	wantsJSON := strings.Contains(r.Header.Get("Accept"), "application/json")

	nfcTagID := r.FormValue("nfc_tag_id")
	if nfcTagID == "" {
		if wantsJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Tag-ID darf nicht leer sein."})
			return
		}
		s.loansWithMessage(w, r, "", "Tag-ID darf nicht leer sein.")
		return
	}

	loan, err := s.store.Return(r.Context(), nfcTagID)
	if err != nil {
		var msg string
		if errors.Is(err, sql.ErrNoRows) {
			msg = "Kein offener Ausleihvorgang für dieses Exemplar gefunden."
		} else {
			_, friendly := s.friendlyError(r, err)
			msg = "Fehler: " + friendly
		}
		if wantsJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]string{"error": msg})
			return
		}
		s.loansWithMessage(w, r, "", msg)
		return
	}

	_ = loan
	if wantsJSON {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}
	s.loansWithMessage(w, r, "Rückgabe erfolgreich erfasst.", "")
}

func (s *Server) loansWithMessage(w http.ResponseWriter, r *http.Request, success, errMsg string) {
	d, err := s.buildLoansData(r, success, errMsg)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if errMsg != "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	render(w, "loans", d)
}

func (s *Server) buildLoansData(r *http.Request, success, errMsg string) (loansData, error) {
	loans, err := s.store.ActiveLoans(r.Context())
	if err != nil {
		return loansData{}, err
	}
	classes, err := s.store.ListClasses(r.Context())
	if err != nil {
		return loansData{}, err
	}
	students, err := s.store.ListStudents(r.Context())
	if err != nil {
		return loansData{}, err
	}

	rows := make([]loanRow, len(loans))
	for i, l := range loans {
		row := loanRow{Loan: l}
		diff := time.Until(l.DueAt).Hours() / 24
		if diff < 0 {
			row.Overdue = true
			row.DaysOverdue = int(-diff)
		} else {
			row.DaysLeft = int(diff)
		}
		if reservations, err := s.store.ActiveReservationsByBook(r.Context(), l.Book.ID); err == nil {
			row.Reservations = reservations
			for _, res := range reservations {
				if res.StudentID != l.StudentID {
					row.ReservedByOther = true
					break
				}
			}
		}
		rows[i] = row
	}

	return loansData{
		Loans:           rows,
		Error:           errMsg,
		Success:         success,
		Classes:         classes,
		StudentsByClass: students,
	}, nil
}

// handleLoanProlong extends an open loan's due date, unless another student
// has an active reservation ("Vormerkung") on the book.
func (s *Server) handleLoanProlong(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect := redirectTarget(r, "/loans")

	dueAt, parseErr := time.ParseInLocation("2006-01-02", r.FormValue("due_date"), time.Local)
	if parseErr != nil {
		redirectWithMessage(w, r, redirect, false, "Bitte ein gültiges Datum wählen.")
		return
	}
	dueAt = dueAt.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	newDue, err := s.store.ProlongLoan(r.Context(), id, dueAt)
	if err != nil {
		_, friendly := s.friendlyError(r, err)
		msg := "Verlängerung fehlgeschlagen: " + friendly
		switch {
		case errors.Is(err, store.ErrLoanReserved):
			msg = "Verlängerung nicht möglich: Ein anderer Schüler hat dieses Buch vorgemerkt."
		case errors.Is(err, store.ErrNotFound):
			msg = "Ausleihe nicht gefunden oder bereits zurückgegeben."
		case errors.Is(err, store.ErrInvalidDueDate):
			msg = "Das neue Rückgabedatum muss in der Zukunft liegen."
		}
		redirectWithMessage(w, r, redirect, false, msg)
		return
	}
	redirectWithMessage(w, r, redirect, true, "Ausleihe verlängert bis "+newDue.Format("02.01.2006")+".")
}

func (s *Server) checkoutFormData(r *http.Request) (checkoutData, error) {
	classes, err := s.store.ListClasses(r.Context())
	if err != nil {
		return checkoutData{}, err
	}
	students, err := s.store.ListStudents(r.Context())
	if err != nil {
		return checkoutData{}, err
	}
	return checkoutData{Classes: classes, StudentsByClass: students}, nil
}
