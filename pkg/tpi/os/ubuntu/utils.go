package ubuntu

import "strings"

// extractLoopDeviceFromKpartx extracts the loop device name from kpartx output
func extractLoopDeviceFromKpartx(kpartxOutput string) string {
	// kpartx output looks like:
	// add map loop0p1 (254:0): 0 xxx xxx /dev/loop0p1
	// add map loop0p2 (254:1): 0 xxx xxx /dev/loop0p2
	lines := strings.Split(kpartxOutput, "\n")
	if len(lines) > 0 {
		fields := strings.Fields(lines[0])
		if len(fields) >= 3 {
			// Extract loop0 from loop0p1
			if dev := fields[2]; strings.HasPrefix(dev, "loop") {
				return strings.TrimSuffix(dev, "p1")
			}
		}
	}
	return ""
}
