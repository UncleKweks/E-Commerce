package services

import (
	"strings"
	"testing"
)

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"shirt":      "shirt",
		"100%":       `100\%`,
		"snake_case": `snake\_case`,
		`back\slash`: `back\\slash`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeSKU(t *testing.T) {
	if got := normalizeSKU("  tshirt-red-m "); got != "TSHIRT-RED-M" {
		t.Errorf("normalizeSKU = %q", got)
	}
}

func TestEverySortEndsWithID(t *testing.T) {
	// Without a unique tiebreaker, rows with equal prices can swap places
	// between page requests, so a product could show on two pages or none.
	for name, order := range productSorts {
		if !strings.HasSuffix(order, ", id ASC") && !strings.HasSuffix(order, ", id DESC") {
			t.Errorf("sort %q = %q does not end with an id tiebreaker", name, order)
		}
	}
	if _, ok := productSorts[DefaultProductSort]; !ok {
		t.Errorf("default sort %q is not in productSorts", DefaultProductSort)
	}
}
