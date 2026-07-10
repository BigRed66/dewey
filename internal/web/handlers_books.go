package web

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/simon/dewey/internal/booksapi"
	"github.com/simon/dewey/internal/store"
)

var categories = []string{
	"Bilderbuch", "Erstleser", "Abenteuer", "Fantasy",
	"Sachbuch", "Natur & Tiere", "Geschichte", "Humor", "Sonstiges",
}

var readingLevels = []string{
	"Klasse 1", "Klasse 1–2", "Klasse 2", "Klasse 2–3",
	"Klasse 3", "Klasse 3–4", "Klasse 4", "Alle",
}

type booksData struct {
	Books []store.Book
}

type booksNewData struct {
	NFCTagID      string
	ISBN          string
	Title         string
	Author        string
	Publisher     string
	PublishedYear int
	CoverURL      string
	Category      string
	ReadingLevel  string
	ShelfRow      string
	ShelfColumn   string
	Categories    []string
	ReadingLevels []string
	// non-zero when this ISBN is already in the DB — only a copy needs adding
	ExistingBookID int64
	LookupError    string
	SaveError      string
}

func emptyNewData() booksNewData {
	return booksNewData{Categories: categories, ReadingLevels: readingLevels}
}

func (s *Server) handleBooksList(w http.ResponseWriter, r *http.Request) {
	books, err := s.store.ListBooks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "books", booksData{Books: books})
}

func (s *Server) handleBooksNew(w http.ResponseWriter, r *http.Request) {
	d := emptyNewData()
	d.NFCTagID = r.URL.Query().Get("nfc_tag_id")
	render(w, "books_new", d)
}

func (s *Server) handleBooksLookup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := emptyNewData()
	d.NFCTagID = r.FormValue("nfc_tag_id")
	d.ISBN = r.FormValue("isbn")

	if d.NFCTagID == "" {
		d.LookupError = "NFC-Tag-ID ist erforderlich."
		render(w, "books_new", d)
		return
	}

	// No ISBN — go straight to the manual form
	if d.ISBN == "" {
		render(w, "books_new", d)
		return
	}

	// Check if this ISBN already exists in the database
	existing, err := s.store.GetBookByISBN(r.Context(), d.ISBN)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		d.LookupError = "Datenbankfehler: " + err.Error()
		render(w, "books_new", d)
		return
	}
	if existing.ID != 0 {
		// Book already exists — just need to add a copy
		d.ExistingBookID = existing.ID
		d.Title = existing.Title
		d.Author = existing.Author
		d.Publisher = existing.Publisher
		d.PublishedYear = existing.PublishedYear
		d.CoverURL = existing.CoverURL
		d.Category = existing.Category
		d.ReadingLevel = existing.ReadingLevel
		d.ShelfRow = existing.ShelfRow
		d.ShelfColumn = existing.ShelfColumn
		render(w, "books_new", d)
		return
	}

	// New book — fetch metadata from Google Books
	apiKey, _ := s.store.GetSetting(r.Context(), "google_books_key")
	info, err := booksapi.Lookup(r.Context(), d.ISBN, apiKey)
	if err != nil {
		d.LookupError = err.Error()
		render(w, "books_new", d)
		return
	}

	d.Title = info.Title
	d.Author = info.Author
	d.Publisher = info.Publisher
	d.PublishedYear = info.PublishedYear
	d.CoverURL = info.CoverURL

	render(w, "books_new", d)
}

type copyDetail struct {
	store.Copy
	OnLoan bool
}

type bookDetailData struct {
	Book            store.Book
	Copies          []copyDetail
	Loans           []loanRow
	CopyCount       int
	OnLoanCount     int
	ShowEditModal   bool
	Reservations    []store.Reservation
	Classes         []store.Class
	StudentsByClass []store.Student
	Error           string
	Success         string
}

func (s *Server) handleBookDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	book, err := s.store.GetBook(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	rawCopies, err := s.store.ListCopiesByBook(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	loans, err := s.store.AllLoansByBook(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reservations, err := s.store.ActiveReservationsByBook(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	classes, err := s.store.ListClasses(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	students, err := s.store.ListStudents(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build set of copy IDs currently on loan
	onLoanSet := map[int64]bool{}
	rows := make([]loanRow, len(loans))
	onLoanCount := 0
	for i, l := range loans {
		row := loanRow{Loan: l}
		if l.ReturnedAt == nil {
			onLoanSet[l.CopyID] = true
			onLoanCount++
			diff := time.Until(l.DueAt).Hours() / 24
			if diff < 0 {
				row.Overdue = true
				row.DaysOverdue = int(-diff)
			} else {
				row.DaysLeft = int(diff)
			}
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

	copies := make([]copyDetail, len(rawCopies))
	for i, c := range rawCopies {
		copies[i] = copyDetail{Copy: c, OnLoan: onLoanSet[c.ID]}
	}

	render(w, "book_detail", bookDetailData{
		Book:            book,
		Copies:          copies,
		Loans:           rows,
		CopyCount:       len(copies),
		OnLoanCount:     onLoanCount,
		ShowEditModal:   r.URL.Query().Get("edit") == "true",
		Reservations:    reservations,
		Classes:         classes,
		StudentsByClass: students,
		Error:           r.URL.Query().Get("error"),
		Success:         r.URL.Query().Get("success"),
	})
}

func (s *Server) handleBooksUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if title == "" {
		http.Redirect(w, r, "/books/"+strconv.FormatInt(id, 10)+"?edit=true", http.StatusSeeOther)
		return
	}
	existing, err := s.store.GetBook(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	year, _ := strconv.Atoi(r.FormValue("published_year"))
	b := store.Book{
		ID:            id,
		ISBN:          existing.ISBN,
		Title:         title,
		Author:        r.FormValue("author"),
		Publisher:     r.FormValue("publisher"),
		PublishedYear: year,
		CoverURL:      existing.CoverURL,
		Category:      r.FormValue("category"),
		ReadingLevel:  r.FormValue("reading_level"),
	}
	if err := s.store.UpdateBook(r.Context(), b); err != nil {
		http.Error(w, "Buch konnte nicht aktualisiert werden: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.SetBookPosition(r.Context(), id, r.FormValue("shelf_row"), r.FormValue("shelf_column")); err != nil {
		http.Error(w, "Buch aktualisiert, aber Position konnte nicht gespeichert werden: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/books/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *Server) handleCopyDelete(w http.ResponseWriter, r *http.Request) {
	copyID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	bookID, _ := strconv.ParseInt(r.FormValue("book_id"), 10, 64)

	onLoan, _ := s.store.CopyOnLoan(r.Context(), copyID)
	if onLoan {
		http.Redirect(w, r, "/books/"+strconv.FormatInt(bookID, 10), http.StatusSeeOther)
		return
	}

	if err := s.store.DeleteCopy(r.Context(), copyID); err != nil {
		http.Error(w, "Exemplar konnte nicht entfernt werden: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If no copies remain, delete the book too
	remaining, _ := s.store.ListCopiesByBook(r.Context(), bookID)
	if len(remaining) == 0 {
		s.store.DeleteBook(r.Context(), bookID)
		http.Redirect(w, r, "/books", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/books/"+strconv.FormatInt(bookID, 10), http.StatusSeeOther)
}

func (s *Server) handleBooksDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.store.DeleteBook(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrBookOnLoan) {
			http.Error(w, "Buch konnte nicht gelöscht werden: Es sind noch Exemplare ausgeliehen.", http.StatusConflict)
			return
		}
		http.Error(w, "Buch konnte nicht gelöscht werden: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/books", http.StatusSeeOther)
}

func (s *Server) handleBooksCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	nfcTagID := r.FormValue("nfc_tag_id")
	if nfcTagID == "" {
		d := emptyNewData()
		d.SaveError = "NFC-Tag-ID ist erforderlich."
		render(w, "books_new", d)
		return
	}

	// If existing_book_id is set, just add a copy to an existing book
	if existingID, _ := strconv.ParseInt(r.FormValue("existing_book_id"), 10, 64); existingID > 0 {
		if _, err := s.store.CreateCopy(r.Context(), existingID, nfcTagID); err != nil {
			book, _ := s.store.GetBook(r.Context(), existingID)
			d := emptyNewData()
			d.NFCTagID = nfcTagID
			d.ExistingBookID = existingID
			d.Title = book.Title
			d.Author = book.Author
			d.CoverURL = book.CoverURL
			d.SaveError = "Exemplar konnte nicht hinzugefügt werden: " + err.Error()
			render(w, "books_new", d)
			return
		}
		http.Redirect(w, r, "/books", http.StatusSeeOther)
		return
	}

	// New book — title is required
	title := r.FormValue("title")
	if title == "" {
		d := emptyNewData()
		d.NFCTagID = nfcTagID
		d.SaveError = "Titel ist erforderlich."
		render(w, "books_new", d)
		return
	}

	year, _ := strconv.Atoi(r.FormValue("published_year"))
	shelfRow := r.FormValue("shelf_row")
	shelfColumn := r.FormValue("shelf_column")
	b := store.Book{
		ISBN:          r.FormValue("isbn"),
		Title:         title,
		Author:        r.FormValue("author"),
		Publisher:     r.FormValue("publisher"),
		PublishedYear: year,
		CoverURL:      r.FormValue("cover_url"),
		Category:      r.FormValue("category"),
		ReadingLevel:  r.FormValue("reading_level"),
	}

	created, err := s.store.CreateBook(r.Context(), b)
	if err != nil {
		d := booksNewData{
			NFCTagID: nfcTagID, ISBN: b.ISBN, Title: b.Title,
			Author: b.Author, Publisher: b.Publisher, PublishedYear: b.PublishedYear,
			CoverURL: b.CoverURL, Category: b.Category, ReadingLevel: b.ReadingLevel,
			ShelfRow: shelfRow, ShelfColumn: shelfColumn,
			Categories: categories, ReadingLevels: readingLevels,
			SaveError: "Buch konnte nicht gespeichert werden: " + err.Error(),
		}
		render(w, "books_new", d)
		return
	}

	if _, err := s.store.CreateCopy(r.Context(), created.ID, nfcTagID); err != nil {
		d := emptyNewData()
		d.NFCTagID = nfcTagID
		d.SaveError = "Buch gespeichert, aber Exemplar konnte nicht hinzugefügt werden: " + err.Error()
		render(w, "books_new", d)
		return
	}

	if shelfRow != "" || shelfColumn != "" {
		if err := s.store.SetBookPosition(r.Context(), created.ID, shelfRow, shelfColumn); err != nil {
			http.Error(w, "Buch gespeichert, aber Position konnte nicht gespeichert werden: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/books", http.StatusSeeOther)
}
