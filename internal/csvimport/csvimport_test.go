package csvimport

import (
	"strings"
	"testing"
)

func TestParseTabSeparated(t *testing.T) {
	in := "Familienname\tVornamen\tKlassenname\n" +
		"Meier\tAnna\t1c\n" +
		"Schulz\tBen Luca\t2a\n"

	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected row errors: %+v", res.Errors)
	}
	if len(res.Students) != 2 {
		t.Fatalf("expected 2 students, got %d (%+v)", len(res.Students), res.Students)
	}

	want := []Student{
		{Line: 2, Name: "Anna Meier", ClassYear: 1, ClassLabel: "C"},
		{Line: 3, Name: "Ben Luca Schulz", ClassYear: 2, ClassLabel: "A"},
	}
	for i, w := range want {
		if res.Students[i] != w {
			t.Errorf("student %d = %+v, want %+v", i, res.Students[i], w)
		}
	}
}

func TestParseSeparatorsAndBOM(t *testing.T) {
	cases := map[string]string{
		"semicolon": "Familienname;Vornamen;Klassenname\nMeier;Anna;1c\n",
		"comma":     "Familienname,Vornamen,Klassenname\nMeier,Anna,1c\n",
		"tab":       "Familienname\tVornamen\tKlassenname\nMeier\tAnna\t1c\n",
		"bom":       "\ufeffFamilienname;Vornamen;Klassenname\nMeier;Anna;1c\n",
		"crlf":      "Familienname;Vornamen;Klassenname\r\nMeier;Anna;1c\r\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := Parse(strings.NewReader(in))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(res.Students) != 1 {
				t.Fatalf("expected 1 student, got %d (errors: %+v)", len(res.Students), res.Errors)
			}
			got := res.Students[0]
			if got.Name != "Anna Meier" || got.ClassYear != 1 || got.ClassLabel != "C" {
				t.Errorf("got %+v", got)
			}
		})
	}
}

func TestParseClassNameVariants(t *testing.T) {
	tests := []struct {
		in        string
		year      int
		label     string
		wantError bool
	}{
		{in: "1c", year: 1, label: "C"},
		{in: "2 B", year: 2, label: "B"},
		{in: " 3a ", year: 3, label: "A"},
		{in: "4", year: 4, label: ""},
		{in: "1C", year: 1, label: "C"},
		{in: "5a", wantError: true},  // outside the schema's 1..4 range
		{in: "0b", wantError: true},  // outside the schema's 1..4 range
		{in: "abc", wantError: true}, // no year
		{in: "", wantError: true},
	}
	for _, tt := range tests {
		year, label, err := parseClass(tt.in)
		if tt.wantError {
			if err == nil {
				t.Errorf("parseClass(%q): expected error, got %d/%q", tt.in, year, label)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseClass(%q): unexpected error %v", tt.in, err)
			continue
		}
		if year != tt.year || label != tt.label {
			t.Errorf("parseClass(%q) = %d/%q, want %d/%q", tt.in, year, label, tt.year, tt.label)
		}
	}
}

// Bad rows must not abort the import — they're reported per line while the
// good rows still come through.
func TestParseCollectsRowErrors(t *testing.T) {
	in := "Familienname;Vornamen;Klassenname\n" +
		"Meier;Anna;1c\n" +
		"Wolf;David;9z\n" + // year out of range
		";;2a\n" + // no name
		"Braun;Greta;\n" + // no class
		"\n" + // blank line, silently skipped
		"Zimmer;Hannah;3a\n"

	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(res.Students) != 2 {
		t.Fatalf("expected 2 usable students, got %d (%+v)", len(res.Students), res.Students)
	}
	if res.Students[0].Name != "Anna Meier" || res.Students[1].Name != "Hannah Zimmer" {
		t.Errorf("unexpected students: %+v", res.Students)
	}

	if len(res.Errors) != 3 {
		t.Fatalf("expected 3 row errors, got %d (%+v)", len(res.Errors), res.Errors)
	}
	wantLines := []int{3, 4, 5}
	for i, want := range wantLines {
		if res.Errors[i].Line != want {
			t.Errorf("error %d on line %d, want %d (%+v)", i, res.Errors[i].Line, want, res.Errors[i])
		}
	}
}

func TestParseMissingColumns(t *testing.T) {
	in := "Vorname;Nachname\nAnna;Meier\n"
	if _, err := Parse(strings.NewReader(in)); err == nil {
		t.Fatal("expected an error when the Klassenname column is missing")
	}
}

func TestParseEmptyFile(t *testing.T) {
	if _, err := Parse(strings.NewReader("   \n")); err == nil {
		t.Fatal("expected an error for an empty file")
	}
}

// Only a header and no data rows is not an error — it just imports nothing.
func TestParseHeaderOnly(t *testing.T) {
	res, err := Parse(strings.NewReader("Familienname;Vornamen;Klassenname\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Students) != 0 || len(res.Errors) != 0 {
		t.Fatalf("expected nothing parsed, got %+v / %+v", res.Students, res.Errors)
	}
}
