package store

import (
	"sort"
	"strings"
)

// sortStudents orders pupils the way a class list is read: by class, then by
// surname, then by given name. Callers get this ordering from every list
// function, so the students page, the class filter and the checkout pickers
// all agree.
//
// Sorting happens in Go rather than SQL because names are stored as a single
// "Vorname Nachname" string, and because SQLite's default collation would
// order umlauts after "z" (putting Müller behind Zimmer).
func sortStudents(students []Student) {
	sort.SliceStable(students, func(i, j int) bool {
		a, b := students[i], students[j]

		if a.Class != nil && b.Class != nil {
			if a.Class.Year != b.Class.Year {
				return a.Class.Year < b.Class.Year
			}
			if a.Class.Label != b.Class.Label {
				return a.Class.Label < b.Class.Label
			}
		}

		if al, bl := sortKey(surname(a.Name)), sortKey(surname(b.Name)); al != bl {
			return al < bl
		}
		return sortKey(a.Name) < sortKey(b.Name)
	})
}

// surname takes the last whitespace-separated word of a name. Names are
// entered and imported as "Vorname Nachname", so for a multi-word surname
// ("Anna von der Heide") this sorts on the final word.
func surname(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// umlautFolder maps umlauts onto their base letters so they sort where a
// German reader expects (DIN 5007-1: ä=a, ö=o, ü=u, ß=ss) instead of after
// "z", which is where a plain byte comparison would put them.
var umlautFolder = strings.NewReplacer(
	"ä", "a", "ö", "o", "ü", "u", "ß", "ss",
)

func sortKey(s string) string {
	return umlautFolder.Replace(strings.ToLower(strings.TrimSpace(s)))
}
