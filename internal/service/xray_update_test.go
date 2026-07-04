package service

import (
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

func TestXrayStatusEmptyPath(t *testing.T) {
	store, err := config.NewStore(t.TempDir() + "/panel.json")
	if err != nil {
		t.Fatal(err)
	}
	p := NewPanelService(store)
	st := p.XrayStatus()
	if st.OK {
		t.Fatal("expected not ok without xray binary")
	}
	if st.Path != "" && st.Version != "" {
		t.Fatalf("unexpected version without binary: %#v", st)
	}
}
