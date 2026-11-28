package logging

import "testing"

func TestSafeFieldEscapesControlCharacters(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"orang-gaboets/repo", "orang-gaboets/repo"},
		{"repo\nverbose: apply completed", `repo\nverbose: apply completed`},
		{"a\rb\x1b[31mc\td", `a\rb\x1b[31mc\td`},
		{`quote"back\slash`, `quote\"back\\slash`},
		{"tëam", "tëam"},
	} {
		if got := SafeField(tt.in); got != tt.want {
			t.Errorf("SafeField(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
