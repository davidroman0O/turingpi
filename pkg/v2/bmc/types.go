package bmc

// PowerState represents the power state of a node
type PowerState string

const (
	PowerStateOn      PowerState = "On"
	PowerStateOff     PowerState = "Off"
	PowerStateUnknown PowerState = "Unknown"
)

// PowerStatus represents the power status of a node
type PowerStatus struct {
	NodeID int
	State  PowerState
}

// BMCInfo represents the BMC information
type BMCInfo struct {
	APIVersion   string
	BuildVersion string
	Buildroot    string
	BuildTime    string
	IPAddress    string
	MACAddress   string
	Version      string
}
