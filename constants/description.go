package constants

// Description describes a VM type offered by a cloud provider.
type Description struct {
	Size   string
	Price  float64
	RAM    float64
	CPU    int
	Disk   string
	Region string
}
