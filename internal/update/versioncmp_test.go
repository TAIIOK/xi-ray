package update

import "testing"

func TestVersionNewer(t *testing.T) {
	tests := []struct {
		a, b   string
		newer  bool
	}{
		{"0.3.0", "0.2.0", true},
		{"0.2.0", "0.3.0", false},
		{"v0.3.0-dirty", "0.2.0", true},
		{"0.3.0-dirty", "v0.2.0", true},
		{"0.2.0", "v0.3.0-dirty", false},
		{"0.3.0", "0.3.0", false},
		{"0.3.1", "0.3.0-dirty", true},
		{"1.0.0", "0.9.9", true},
		{"dev", "0.2.0", false},
		{"0.2.0", "dev", true},
	}
	for _, tc := range tests {
		got := VersionNewer(tc.a, tc.b)
		if got != tc.newer {
			t.Errorf("VersionNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.newer)
		}
	}
}

func TestIsUpdateAvailable(t *testing.T) {
	if IsUpdateAvailable("0.2.0", "v0.3.0-dirty") {
		t.Fatal("0.2.0 should not be available when running 0.3.0-dirty")
	}
	if !IsUpdateAvailable("0.4.0", "0.3.0-dirty") {
		t.Fatal("0.4.0 should be available when running 0.3.0-dirty")
	}
}
