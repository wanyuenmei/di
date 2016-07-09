package provider

import (
	"github.com/NetSys/quilt/constants"
	"github.com/NetSys/quilt/stitch"
)

// XXX: cache results?
func pickBestSize(descriptions []constants.Description, ram stitch.Range,
	cpu stitch.Range, maxPrice float64) string {
	var best constants.Description
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
