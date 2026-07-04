package service

// DetectWatchdogOutage returns whether the watchdog would treat the system as in outage
// and a short machine-readable reason string.
func DetectWatchdogOutage(xrayRunning, vpnConnected bool, obs ObservatoryStatus) (bool, string) {
	if !xrayRunning {
		return true, "xray not running"
	}
	if vpnConnected {
		return false, ""
	}
	allDead := len(obs.Nodes) > 0
	for _, n := range obs.Nodes {
		if n.Alive {
			allDead = false
			break
		}
	}
	if allDead || len(obs.Nodes) == 0 {
		return true, "VPN probe failed and all outbounds dead"
	}
	return false, ""
}
