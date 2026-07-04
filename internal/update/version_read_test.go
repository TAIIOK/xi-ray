package update

import "testing"

func TestParsePanelVersionLine(t *testing.T) {
	got := parsePanelVersionLine("xiaomi-vless v0.2.0 (abc123, 2026-07-04T10:00:00Z)")
	if got != "v0.2.0" {
		t.Fatalf("got %q", got)
	}
	got = parsePanelVersionLine("xiaomi-vless v0.3.0-dirty (abc, date)")
	if got != "v0.3.0-dirty" {
		t.Fatalf("got %q", got)
	}
}

func TestSameVersionLabel(t *testing.T) {
	if !SameVersionLabel("v0.3.2", "0.3.2") {
		t.Fatal("expected same")
	}
	if SameVersionLabel("v0.3.0-dirty", "v0.3.2") {
		t.Fatal("expected different")
	}
}
