// Package csvimport parses pupil lists exported from school administration
// software (columns Familienname / Vornamen / Klassenname) into rows ready
// to be written to the database.
package csvimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// MaxBytes caps how much of an uploaded file is read, so a huge or bogus
// file can't exhaust memory.
const MaxBytes = 4 << 20 // 4 MiB

// Student is one successfully parsed row.
type Student struct {
	Line       int
	Name       string
	ClassYear  int
	ClassLabel string
}

// RowError explains why a single row was unusable. Bad rows are collected
// rather than aborting the whole file, so one typo doesn't block an import
// of several hundred pupils — the UI lists them for correction.
type RowError struct {
	Line    int
	Message string
}

// Result holds the usable rows plus per-row problems.
type Result struct {
	Students []Student
	Errors   []RowError
}

// Column headers are matched case-insensitively; the first alias found wins.
var (
	lastNameAliases  = []string{"familienname", "nachname", "name"}
	firstNameAliases = []string{"vornamen", "vorname"}
	classAliases     = []string{"klassenname", "klasse"}
)

// classRe matches a class name like "1c", "2 B" or "3" — a year number
// followed by an optional letter identifier.
var classRe = regexp.MustCompile(`^(\d{1,2})([\p{L}][\p{L}\d]*)?$`)

// Parse reads the CSV/TSV in r. It auto-detects tab, semicolon and comma
// separators (Excel exports differ by locale) and tolerates a UTF-8 BOM.
func Parse(r io.Reader) (Result, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxBytes))
	if err != nil {
		return Result{}, fmt.Errorf("Datei konnte nicht gelesen werden: %w", err)
	}

	text := strings.TrimPrefix(string(raw), "\ufeff")
	if strings.TrimSpace(text) == "" {
		return Result{}, errors.New("Die Datei ist leer.")
	}

	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = detectSeparator(text)
	cr.FieldsPerRecord = -1 // rows may have trailing empty columns
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	records, err := cr.ReadAll()
	if err != nil {
		return Result{}, fmt.Errorf("Datei konnte nicht gelesen werden: %w", err)
	}
	if len(records) == 0 {
		return Result{}, errors.New("Die Datei ist leer.")
	}

	lastIdx, firstIdx, classIdx, err := headerIndexes(records[0])
	if err != nil {
		return Result{}, err
	}

	var res Result
	for i, rec := range records[1:] {
		line := i + 2 // 1-based, and the header is line 1

		if isBlank(rec) {
			continue
		}

		name := fullName(field(rec, firstIdx), field(rec, lastIdx))
		if name == "" {
			res.Errors = append(res.Errors, RowError{line, "Kein Name angegeben."})
			continue
		}

		year, label, err := parseClass(field(rec, classIdx))
		if err != nil {
			res.Errors = append(res.Errors, RowError{line, err.Error()})
			continue
		}

		res.Students = append(res.Students, Student{
			Line:       line,
			Name:       name,
			ClassYear:  year,
			ClassLabel: label,
		})
	}

	return res, nil
}

// detectSeparator picks whichever candidate separator occurs most often in
// the header line.
func detectSeparator(text string) rune {
	header := text
	if i := strings.IndexAny(header, "\r\n"); i >= 0 {
		header = header[:i]
	}
	best, bestCount := ';', -1
	for _, sep := range []rune{'\t', ';', ','} {
		if n := strings.Count(header, string(sep)); n > bestCount {
			best, bestCount = sep, n
		}
	}
	return best
}

func headerIndexes(header []string) (last, first, class int, err error) {
	norm := make([]string, len(header))
	for i, h := range header {
		norm[i] = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
	}

	last = indexOfAny(norm, lastNameAliases)
	first = indexOfAny(norm, firstNameAliases)
	class = indexOfAny(norm, classAliases)

	var missing []string
	if last < 0 && first < 0 {
		missing = append(missing, "Familienname/Vornamen")
	}
	if class < 0 {
		missing = append(missing, "Klassenname")
	}
	if len(missing) > 0 {
		return 0, 0, 0, fmt.Errorf(
			"Spalten fehlen: %s. Erwartet werden die Spalten Familienname, Vornamen und Klassenname.",
			strings.Join(missing, ", "))
	}
	return last, first, class, nil
}

func indexOfAny(header, aliases []string) int {
	for _, alias := range aliases {
		for i, h := range header {
			if h == alias {
				return i
			}
		}
	}
	return -1
}

func field(rec []string, idx int) string {
	if idx < 0 || idx >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[idx])
}

// fullName renders "Vornamen Familienname", tolerating either part missing.
func fullName(first, last string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(first+" "+last), " "))
}

func parseClass(raw string) (year int, label string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, "", errors.New("Kein Klassenname angegeben.")
	}
	// "1 c" and "1c" mean the same class.
	compact := strings.Join(strings.Fields(s), "")

	m := classRe.FindStringSubmatch(compact)
	if m == nil {
		return 0, "", fmt.Errorf("Klassenname %q nicht verstanden (erwartet z. B. %q).", raw, "1c")
	}

	year, _ = strconv.Atoi(m[1])
	if year < 1 || year > 4 {
		return 0, "", fmt.Errorf("Jahrgang %d aus %q liegt nicht zwischen 1 und 4.", year, raw)
	}
	return year, strings.ToUpper(m[2]), nil
}

func isBlank(rec []string) bool {
	for _, f := range rec {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}
