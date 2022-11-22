package main

import (
	"math"
	"net"
	"time"

	"github.com/yl2chen/cidranger"
)

type glineData struct {
	ipNet     net.IPNet
	user      string
	mask      string
	expireTS  int64
	lastModTS int64
	//TTL       int64
}

func Is_valid_ip(ip string) bool {
	if r := net.ParseIP(ip); r == nil {
		return false
	}
	return true
}

func Is_valid_cidr(cidr string) bool {
	if _, _, r := net.ParseCIDR(cidr); r == nil {
		return false
	}
	return true
}

// get function for network
func (b *glineData) Network() net.IPNet {
	return b.ipNet
}

// get function for network converted to string
func (b *glineData) NetworkStr() string {
	return b.ipNet.String()
}

func (g *glineData) Mask() string {
	return g.mask
}

func (g *glineData) ExpireTS() int64 {
	return g.expireTS
}

func (g *glineData) HoursUntilExpiration() int64 {
	// If the gline expires in 1 hour and 1 second, this function will return 2
	return int64(math.Ceil(float64((g.ExpireTS() - time.Now().Unix())) / 3600.0))
}

func (g *glineData) IsGlineActive() bool {
	return (g.ExpireTS() > int64(time.Now().Unix()))
}

// create customRangerEntry object using net and asn
func newGlineData(ipNet net.IPNet, user, mask string, expireTS, lastModTS int64) cidranger.RangerEntry {
	return &glineData{
		ipNet:     ipNet,
		user:      user,
		mask:      mask,
		lastModTS: lastModTS,
		expireTS:  expireTS,
		//TTL:       TTL,
	}
}
