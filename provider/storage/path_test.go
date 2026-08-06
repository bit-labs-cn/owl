package storage

import "testing"

func TestNormalizeObjectKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/template/a.xlsx", "template/a.xlsx"},
		{"\\template\\a.xlsx", "template/a.xlsx"},
		{"  template/a.xlsx  ", "template/a.xlsx"},
		{"template//a.xlsx", "template/a.xlsx"},
		{"template/a.xlsx/", "template/a.xlsx"},
	}
	for _, c := range cases {
		if got := NormalizeObjectKey(c.in); got != c.want {
			t.Fatalf("NormalizeObjectKey(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
