package os

// OSIdentifier uniquely identifies an operating system and its version.
type OSIdentifier struct {
	Type    string // e.g., "ubuntu"
	Version string // e.g., "22.04"
}

// String returns a string representation of the OS identifier.
func (id OSIdentifier) String() string {
	return id.Type + "@" + id.Version
}
