package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/taiiok/xiaomi-vless/internal/config"
)

const (
	GuestBridge      = "br-guest"
	MainLANBridge    = "br-lan"
)

// GuestNetworkStatus describes guest Wi‑Fi detection on the router.
type GuestNetworkStatus struct {
	OK               bool   `json:"ok"`
	IsGuest          bool   `json:"is_guest"`
	Warning          string `json:"warning,omitempty"`
	Message          string `json:"message"`
	ConfigSubnet     string `json:"config_subnet"`
	DetectedSubnet   string `json:"detected_subnet,omitempty"`
	DetectedGateway  string `json:"detected_gateway,omitempty"`
	Interface        string `json:"interface,omitempty"`
	MainLANSubnet    string `json:"main_lan_subnet,omitempty"`
	SuggestedSubnet  string `json:"suggested_subnet,omitempty"`
	GuestInterfaceUp bool   `json:"guest_interface_up"`
}

type bridgeAddr struct {
	name    string
	up      bool
	ipv4    string // e.g. 192.168.33.1/24
	gateway string // host part without mask
	subnet  string // network CIDR e.g. 192.168.33.0/24
}

// DetectGuest checks br-guest/br-lan against the configured guest subnet.
func DetectGuest(configSubnet string) GuestNetworkStatus {
	if configSubnet == "" {
		configSubnet = config.DefaultGuestSubnet
	}
	st := GuestNetworkStatus{ConfigSubnet: configSubnet}

	if err := config.ValidateGuestSubnet(configSubnet); err != nil {
		st.Message = "Некорректная подсеть в конфиге: " + err.Error()
		st.Warning = st.Message
		return st
	}

	out, err := exec.Command("ip", "-br", "addr", "show").Output()
	if err != nil {
		st.Message = "Не удалось опросить интерфейсы (ip addr): " + err.Error()
		st.Warning = st.Message
		return st
	}

	guest, lan := parseBriefAddrs(string(out))
	if lan.subnet != "" {
		st.MainLANSubnet = lan.subnet
	}

	if overlapsSubnet(configSubnet, config.DefaultMainLANSubnet) || (lan.subnet != "" && subnetsEqual(configSubnet, lan.subnet)) {
		st.IsGuest = false
		st.Warning = "Указана подсеть основной сети. Использование VPN на основной LAN — на свой страх и риск, возможна потеря интернета для всех устройств."
		st.Message = st.Warning
		return st
	}

	if guest.name == "" || guest.ipv4 == "" {
		st.IsGuest = false
		st.Message = "Гостевая Wi‑Fi не обнаружена — включите её в настройках роутера"
		st.Warning = st.Message
		return st
	}

	st.Interface = guest.name
	st.GuestInterfaceUp = guest.up
	st.DetectedGateway = guest.gateway
	st.DetectedSubnet = guest.subnet

	if !guest.up {
		st.IsGuest = false
		st.Message = "Интерфейс br-guest найден, но не активен (DOWN)"
		st.Warning = st.Message
		if guest.subnet != "" {
			st.SuggestedSubnet = guest.subnet
		}
		return st
	}

	if guest.subnet != "" && !subnetsEqual(configSubnet, guest.subnet) {
		st.IsGuest = true
		st.SuggestedSubnet = guest.subnet
		st.Message = fmt.Sprintf("Подсеть в конфиге (%s) не совпадает с br-guest (%s)", configSubnet, guest.subnet)
		st.Warning = "Нажмите «Подставить с роутера» или измените подсеть вручную"
		return st
	}

	st.OK = true
	st.IsGuest = true
	st.Message = fmt.Sprintf("Гостевая сеть OK — %s на %s", guest.subnet, guest.name)
	return st
}

// DetectGuestFromOutput is used in tests with fixture ip output.
func DetectGuestFromOutput(configSubnet, ipOutput string) GuestNetworkStatus {
	if configSubnet == "" {
		configSubnet = config.DefaultGuestSubnet
	}
	st := GuestNetworkStatus{ConfigSubnet: configSubnet}

	if err := config.ValidateGuestSubnet(configSubnet); err != nil {
		st.Message = "Некорректная подсеть в конфиге: " + err.Error()
		st.Warning = st.Message
		return st
	}

	guest, lan := parseBriefAddrs(ipOutput)
	if lan.subnet != "" {
		st.MainLANSubnet = lan.subnet
	}

	if overlapsSubnet(configSubnet, config.DefaultMainLANSubnet) || (lan.subnet != "" && subnetsEqual(configSubnet, lan.subnet)) {
		st.IsGuest = false
		st.Warning = "Указана подсеть основной сети. Использование VPN на основной LAN — на свой страх и риск, возможна потеря интернета для всех устройств."
		st.Message = st.Warning
		return st
	}

	if guest.name == "" || guest.ipv4 == "" {
		st.IsGuest = false
		st.Message = "Гостевая Wi‑Fi не обнаружена — включите её в настройках роутера"
		st.Warning = st.Message
		return st
	}

	st.Interface = guest.name
	st.GuestInterfaceUp = guest.up
	st.DetectedGateway = guest.gateway
	st.DetectedSubnet = guest.subnet

	if !guest.up {
		st.IsGuest = false
		st.Message = "Интерфейс br-guest найден, но не активен (DOWN)"
		st.Warning = st.Message
		if guest.subnet != "" {
			st.SuggestedSubnet = guest.subnet
		}
		return st
	}

	if guest.subnet != "" && !subnetsEqual(configSubnet, guest.subnet) {
		st.IsGuest = true
		st.SuggestedSubnet = guest.subnet
		st.Message = fmt.Sprintf("Подсеть в конфиге (%s) не совпадает с br-guest (%s)", configSubnet, guest.subnet)
		st.Warning = "Нажмите «Подставить с роутера» или измените подсеть вручную"
		return st
	}

	st.OK = true
	st.IsGuest = true
	st.Message = fmt.Sprintf("Гостевая сеть OK — %s на %s", guest.subnet, guest.name)
	return st
}

func parseBriefAddrs(output string) (guest, lan bridgeAddr) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		state := parts[1]
		ba := bridgeAddr{name: name, up: linkLooksUp(state)}
		for _, f := range parts[2:] {
			if !strings.Contains(f, ".") || strings.Contains(f, ":") {
				continue
			}
			if ip, subnet, ok := parseIPv4CIDR(f); ok {
				ba.ipv4 = f
				ba.gateway = ip
				ba.subnet = subnet
				break
			}
		}
		switch name {
		case GuestBridge:
			guest = ba
		case MainLANBridge:
			lan = ba
		}
	}
	return guest, lan
}

func linkLooksUp(state string) bool {
	u := strings.ToUpper(strings.TrimSpace(state))
	if u == "DOWN" {
		return false
	}
	// Linux bridges without a carrier report UNKNOWN even when configured (common in QEMU lab).
	return strings.Contains(u, "UP") || u == "UNKNOWN"
}

func parseIPv4CIDR(s string) (ip, network string, ok bool) {
	ipStr, ipNet, err := net.ParseCIDR(s)
	if err != nil || ipStr.To4() == nil {
		return "", "", false
	}
	return ipStr.String(), ipNet.String(), true
}

func subnetsEqual(a, b string) bool {
	_, na, errA := net.ParseCIDR(strings.TrimSpace(a))
	_, nb, errB := net.ParseCIDR(strings.TrimSpace(b))
	if errA != nil || errB != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return na.IP.Equal(nb.IP) && na.Mask.String() == nb.Mask.String()
}

func overlapsSubnet(a, b string) bool {
	_, na, errA := net.ParseCIDR(strings.TrimSpace(a))
	_, nb, errB := net.ParseCIDR(strings.TrimSpace(b))
	if errA != nil || errB != nil {
		return false
	}
	return na.Contains(nb.IP) || nb.Contains(na.IP)
}
