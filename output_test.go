package main

import "testing"

func TestNum(t *testing.T) {
	cases := map[int]string{
		0: "0", 7: "7", 999: "999", 1000: "1,000",
		9284: "9,284", 1000000: "1,000,000", -1234: "-1,234",
	}
	for n, want := range cases {
		if got := num(n); got != want {
			t.Errorf("num(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	if got := plural(1, "row", "rows"); got != "1 row" {
		t.Errorf("plural(1) = %q, want %q", got, "1 row")
	}
	if got := plural(0, "row", "rows"); got != "0 rows" {
		t.Errorf("plural(0) = %q, want %q", got, "0 rows")
	}
	if got := plural(12, "row", "rows"); got != "12 rows" {
		t.Errorf("plural(12) = %q, want %q", got, "12 rows")
	}
}

func TestStatusTagIsFixedWidth(t *testing.T) {
	useColor = false
	defer func() { useColor = false }()

	cases := map[string]string{
		StatusExact:     "✓ EXACT        ",
		StatusAutomatic: "≈ AUTOMATIC    ",
		StatusManual:    "★ MANUAL       ",
		StatusNotFound:  "✗ NOT FOUND    ",
		StatusSkipped:   "– SKIPPED      ",
		StatusDuplicate: "= DUPLICATE    ",
		StatusDiscarded: "− DISCARDED    ",
	}
	for status, want := range cases {
		if got := statusTag(status); got != want {
			t.Errorf("statusTag(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestDescribeRow(t *testing.T) {
	r := Row{Track: "Gianna", Artist: "Rino Gaetano", Album: "Anni 70"}
	if got, want := describeRow(r), "«Gianna» — Rino Gaetano [ Anni 70 ]"; got != want {
		t.Errorf("describeRow() = %q, want %q", got, want)
	}
	r = Row{Track: "Gianna"}
	if got, want := describeRow(r), "«Gianna»"; got != want {
		t.Errorf("describeRow() without artist/album = %q, want %q", got, want)
	}
}
