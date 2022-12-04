package main

import (
	"fmt"
	"log"
	"math"
	"net"
	"strings"
	"time"

	"github.com/yl2chen/cidranger"
)

type glineData struct {
	ipNet     net.IPNet
	user      string
	mask      string
	reason    string
	expireTS  int64
	lastModTS int64
	active    bool
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

func (g *glineData) LastModTS() int64 {
	return g.lastModTS
}

func (g *glineData) HoursUntilExpiration() int64 {
	// If the gline expires in 1 hour and 1 second, this function will return 2
	return int64(math.Ceil(float64((g.ExpireTS() - time.Now().Unix())) / 3600.0))
}

func (g *glineData) HoursSinceLastMod() int64 {
	// If the gline expires in 1 hour and 1 second, this function will return 2
	return int64(math.Ceil(float64((time.Now().Unix() - g.LastModTS())) / 3600.0))
}

func (g *glineData) IsGlineActive() bool {
	return g.active && (g.ExpireTS() > int64(time.Now().Unix()))
}

// Updates glineData. If expireTS=0, expireTS value is not modified.
func (g *glineData) UpdateGline(active bool, expireTS int64) {
	g.lastModTS = time.Now().Unix()
	g.active = active
	if expireTS != 0 {
		g.expireTS = expireTS
	}
	//fmt.Println("DEBUG: UpdateGline:", g.mask, active, expireTS)
}

// create customRangerEntry object using net and asn
func newGlineData(ipNet net.IPNet, user, mask string, expireTS, lastModTS int64, reason string, active bool) cidranger.RangerEntry {
	return &glineData{
		ipNet:     ipNet,
		user:      user,
		mask:      mask,
		lastModTS: lastModTS,
		expireTS:  expireTS,
		active:    active,
		reason:    reason,
		//TTL:       TTL,
	}
}

// Updates existing glineData information based on gline mask.
// Returns true if ip gline mask exists in current glineData struct. False otherwise.
func (s *serverData) UpdateGline(mask string, active bool, expireTS int64) bool {
	mask_l := strings.Split(mask, "@")
	if len(mask_l) < 2 {
		return false
	}
	ip := mask_l[1]
	ip = StripCidrFromIP(ip)
	log.Printf("DEBUG: serverData.UpdateGline(): ip=%s\n", ip)
	if glines, exp_glines, err := s.CheckGline(ip); err == nil {
		for _, entry := range glines {
			emask := entry.Mask()
			log.Printf("DEBUG: serverData.UpdateGline(): Comparing %s and %s in glines\n", mask, emask)
			if strings.EqualFold(mask, emask) {
				entry.UpdateGline(active, expireTS)
				return true
			}
		}
		for _, entry := range exp_glines {
			emask := entry.Mask()
			log.Printf("DEBUG: serverData.UpdateGline(): Comparing %s and %s in exp_glines\n", mask, emask)
			if strings.EqualFold(mask, emask) {
				entry.UpdateGline(active, expireTS)
				return true
			}
		}
	} else {
		fmt.Println("Error in serverData.UpdateGline(): invalid mask provided")
	}
	return false
}

// This method accepts an IP as parameter and returns two lists:
// active glines and expired/deactivated glines.
// An error is returned if the IP is invalid
func (s *serverData) CheckGline(ip string) ([]*glineData, []*glineData, error) {
	// request networks containing this IP
	entries, err := s.cranger.ContainingNetworks(net.ParseIP(ip))
	if err != nil {
		log.Printf("Debug: serverData.CheckGline(): ip=%s, error = %s\n", ip, err.Error())
	}
	activeGlines := make([]*glineData, 0, len(entries))
	inactiveGlines := make([]*glineData, 0, len(entries))
	/*if err != nil {
		fmt.Println("ranger.ContainingNetworks()", err.Error())
		os.Exit(1)
	}*/

	//TODO: Remove the lines below, which is there for debug purposes only
	//log.Printf("Entries for %s:\n", ip)
	for _, e := range entries {

		// Cast e (cidranger.RangerEntry to struct glineData
		entry, ok := e.(*glineData)
		if !ok {
			log.Fatalln("This shouldn't have happened")
			continue
		}
		if entry.IsGlineActive() {
			activeGlines = append(activeGlines, entry)
		} else {
			inactiveGlines = append(inactiveGlines, entry)
		}

		/*
			// Get network (converted to string by function)
			n := entry.NetworkStr()

			// Get mask
			mask := entry.Mask()

			// Display
			fmt.Println("\t", n, mask)
		*/
	}
	return activeGlines, inactiveGlines, err
}
