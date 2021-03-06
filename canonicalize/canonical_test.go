package canonicalize

import (
	"strings"
	"testing"

	"github.com/nelhage/taktician/ptn"
	"github.com/nelhage/taktician/tak"
)

func TestCanonical(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"a1", "a1"},
		{"a5", "a1"},
		{"e5", "a1"},
		{"e1", "a1"},

		{"a1 a5", "a1 e1"},
		{"a5 e5", "a1 e1"},
		{"e5 e1", "a1 e1"},
		{"e1 a1", "a1 e1"},

		{"a5 a1", "a1 e1"},
		{"a1 e1", "a1 e1"},
		{"e1 e5", "a1 e1"},
		{"e5 a5", "a1 e1"},

		{"e5 a1", "a1 e5"},
		{"a1 e5", "a1 e5"},

		{"a5 e1", "a1 e5"},
		{"e1 a5", "a1 e5"},

		{"a1 e5 b4", "a1 e5 d2"},

		{"a1 a5 e5 e1 c4 b4", "a1 e1 e5 a5 d3 d2"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			bits := strings.Split(tc.in, " ")
			var ms []tak.Move
			for _, b := range bits {
				m, e := ptn.ParseMove(b)
				if e != nil {
					t.Fatalf("Parse %s: %v", b, e)
				}
				ms = append(ms, m)
			}
			out, _ := Canonical(5, ms)
			bits = nil
			for _, o := range out {
				bits = append(bits, ptn.FormatMove(&o))
			}
			got := strings.Join(bits, " ")
			if got != tc.out {
				t.Fatalf("Canonical(%q) = %q != %q",
					tc.in, got, tc.out,
				)
			}
		})
	}
}
