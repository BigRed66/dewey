package store

import "testing"

func names(students []Student) []string {
	out := make([]string, len(students))
	for i, s := range students {
		out[i] = s.Name
	}
	return out
}

func studentsNamed(names ...string) []Student {
	out := make([]Student, len(names))
	for i, n := range names {
		out[i] = Student{Name: n}
	}
	return out
}

func assertOrder(t *testing.T, got []Student, want ...string) {
	t.Helper()
	gotNames := names(got)
	if len(gotNames) != len(want) {
		t.Fatalf("got %v, want %v", gotNames, want)
	}
	for i := range want {
		if gotNames[i] != want[i] {
			t.Fatalf("got %v, want %v", gotNames, want)
		}
	}
}

func TestSortStudentsBySurname(t *testing.T) {
	s := studentsNamed("Anna Zimmer", "Ben Meier", "Clara Braun")
	sortStudents(s)
	assertOrder(t, s, "Clara Braun", "Ben Meier", "Anna Zimmer")
}

// Given names only break ties within the same surname.
func TestSortStudentsSameSurname(t *testing.T) {
	s := studentsNamed("Marit Sonnenberg", "Jonne Sonnenberg", "Ada Sonnenberg")
	sortStudents(s)
	assertOrder(t, s, "Ada Sonnenberg", "Jonne Sonnenberg", "Marit Sonnenberg")
}

// A plain byte comparison would sort every umlaut after "z"; German readers
// expect Müller between Meier and Nowak.
func TestSortStudentsUmlauts(t *testing.T) {
	s := studentsNamed("Anna Zimmer", "Tim Müller", "Eva Meier", "Jan Nowak", "Lea Schäfer")
	sortStudents(s)
	assertOrder(t, s, "Eva Meier", "Tim Müller", "Jan Nowak", "Lea Schäfer", "Anna Zimmer")
}

func TestSortStudentsMultiWordNames(t *testing.T) {
	// Surname is the final word, so "von der Heide" sorts under H.
	s := studentsNamed("Anna Zimmer", "Ben Luca Adler", "Clara von der Heide")
	sortStudents(s)
	assertOrder(t, s, "Ben Luca Adler", "Clara von der Heide", "Anna Zimmer")
}

// The students page groups by class, so class ordering wins over the name.
func TestSortStudentsGroupsByClassFirst(t *testing.T) {
	s := []Student{
		{Name: "Anna Adler", Class: &Class{Year: 2, Label: "A"}},
		{Name: "Zoe Zimmer", Class: &Class{Year: 1, Label: "A"}},
		{Name: "Ben Braun", Class: &Class{Year: 1, Label: "B"}},
		{Name: "Ada Auer", Class: &Class{Year: 1, Label: "A"}},
	}
	sortStudents(s)
	assertOrder(t, s, "Ada Auer", "Zoe Zimmer", "Ben Braun", "Anna Adler")
}

func TestSurname(t *testing.T) {
	tests := map[string]string{
		"Anna Meier":         "Meier",
		"Ben Luca Schulz":    "Schulz",
		"Cher":               "Cher",
		"  Extra   Spaces  ": "Spaces",
		"":                   "",
	}
	for in, want := range tests {
		if got := surname(in); got != want {
			t.Errorf("surname(%q) = %q, want %q", in, got, want)
		}
	}
}
