package provider

import (
	"github.com/NetSys/quilt/stitch"
)

// Description describes a VM type offered by a cloud provider
type Description struct {
	Size   string
	Price  float64
	RAM    float64
	CPU    int
	Disk   string
	Region string
}

// XXX: cache results?
func pickBestSize(descriptions []Description, ram stitch.Range, cpu stitch.Range,
	maxPrice float64) string {
	var best Description
	for _, d := range descriptions {
		if ram.Accepts(d.RAM) &&
			cpu.Accepts(float64(d.CPU)) &&
			(best.Size == "" || d.Price < best.Price) {
			best = d
		}
	}
	if maxPrice == 0 || best.Price <= maxPrice {
		return best.Size
	}
	return ""
}
