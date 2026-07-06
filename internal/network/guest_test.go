package network

import (
	"testing"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

const fixtureGuestOK = `br-lan           UP             192.168.31.1/24 fe80::1/64 scope link
br-guest         UP             192.168.33.1/24 fe80::2/64 scope link
`

const fixtureGuestMissing = `br-lan           UP             192.168.31.1/24
`

const fixtureGuestMismatch = `br-lan           UP             192.168.31.1/24
br-guest         UP             192.168.34.1/24
`

const fixtureGuestDown = `br-lan           UP             192.168.31.1/24
br-guest         DOWN
`

const fixtureGuestUnknown = `br-lan           UP             192.168.1.1/24
br-guest         UNKNOWN        192.168.33.1/24
`

func TestDetectGuestFromOutput_UnknownBridgeUp(t *testing.T) {
	st := DetectGuestFromOutput(config.DefaultGuestSubnet, fixtureGuestUnknown)
	if !st.OK {
		t.Fatalf("expected ok for UNKNOWN bridge with IP: %+v", st)
	}
	if !st.IsGuest {
		t.Fatal("expected is_guest")
	}
}

func TestDetectGuestFromOutput_OK(t *testing.T) {
	st := DetectGuestFromOutput(config.DefaultGuestSubnet, fixtureGuestOK)
	if !st.OK {
		t.Fatalf("expected ok: %+v", st)
	}
	if !st.IsGuest {
		t.Fatal("expected is_guest")
	}
	if st.DetectedGateway != "192.168.33.1" {
		t.Fatalf("gateway = %q", st.DetectedGateway)
	}
	if st.DetectedSubnet != "192.168.33.0/24" {
		t.Fatalf("detected subnet = %q", st.DetectedSubnet)
	}
	if st.MainLANSubnet != "192.168.31.0/24" {
		t.Fatalf("main lan = %q", st.MainLANSubnet)
	}
}

func TestDetectGuestFromOutput_Missing(t *testing.T) {
	st := DetectGuestFromOutput(config.DefaultGuestSubnet, fixtureGuestMissing)
	if st.OK {
		t.Fatal("expected not ok")
	}
	if st.IsGuest {
		t.Fatal("expected not guest")
	}
}

func TestDetectGuestFromOutput_Mismatch(t *testing.T) {
	st := DetectGuestFromOutput(config.DefaultGuestSubnet, fixtureGuestMismatch)
	if st.OK {
		t.Fatal("expected not ok")
	}
	if st.SuggestedSubnet != "192.168.34.0/24" {
		t.Fatalf("suggested = %q", st.SuggestedSubnet)
	}
}

func TestDetectGuestFromOutput_MainLANConfig(t *testing.T) {
	st := DetectGuestFromOutput(config.DefaultMainLANSubnet, fixtureGuestOK)
	if st.IsGuest {
		t.Fatal("main lan config should not be guest")
	}
	if st.Warning == "" {
		t.Fatal("expected warning")
	}
}

func TestDetectGuestFromOutput_GuestDown(t *testing.T) {
	st := DetectGuestFromOutput(config.DefaultGuestSubnet, fixtureGuestDown)
	if st.OK {
		t.Fatal("expected not ok when guest down")
	}
}

func TestParseBriefAddrs(t *testing.T) {
	guest, lan := parseBriefAddrs(fixtureGuestOK)
	if guest.name != "br-guest" || !guest.up || guest.gateway != "192.168.33.1" {
		t.Fatalf("guest: %+v", guest)
	}
	if lan.subnet != "192.168.31.0/24" {
		t.Fatalf("lan: %+v", lan)
	}
}

func TestSubnetsEqual(t *testing.T) {
	if !subnetsEqual("192.168.33.0/24", "192.168.33.0/24") {
		t.Fatal("expected equal")
	}
	if subnetsEqual("192.168.33.0/24", "192.168.34.0/24") {
		t.Fatal("expected not equal")
	}
}

func TestOverlapsSubnet(t *testing.T) {
	if !overlapsSubnet("192.168.31.0/24", "192.168.31.128/25") {
		t.Fatal("expected overlap")
	}
	if overlapsSubnet("192.168.33.0/24", "192.168.31.0/24") {
		t.Fatal("expected no overlap")
	}
}

func TestValidateGuestSubnet(t *testing.T) {
	if err := config.ValidateGuestSubnet("192.168.33.0/24"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "not-a-cidr", "2001:db8::/32"} {
		if err := config.ValidateGuestSubnet(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	norm, err := config.NormalizeGuestSubnet("192.168.33.1/24")
	if err != nil || norm != "192.168.33.0/24" {
		t.Fatalf("normalize host cidr: %q err=%v", norm, err)
	}
}
