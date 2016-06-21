package provider

import (
	"github.com/NetSys/quilt/stitch"
)

type description struct {
	size   string
	price  float64
	ram    float64
	cpu    int
	disk   string
	region string
}

// XXX: cache results?
func pickBestSize(descriptions []description, ram stitch.Range, cpu stitch.Range,
	maxPrice float64) string {
	var best description
	for _, d := range descriptions {
		if ram.Accepts(d.ram) && cpu.Accepts(float64(d.cpu)) &&
			(best.size == "" || d.price < best.price) {
			best = d
		}
	}
	if maxPrice == 0 || best.price <= maxPrice {
		return best.size
	}
	return ""
}
