package ircglineapi

import (
	"log"
	"math"
	"net"
	"strings"
	"time"

	"github.com/hiddn/cidranger"
)

type glineData struct {
	ipNet     net.IPNet
	user      string
	mask      string
	reason    string
	expireTS  int64
	lastModTS int64
	active    bool
}

type glinesData struct {
	IpNet  net.IPNet
	Glines []*glineData
}

func Is_valid_ip(ip string) bool {
	if r := net.ParseIP(ip); r == nil {
		return false
	}
	return true
}

func Is_valid_cidr(cidr string) bool {
	if _, _, r := net.ParseCIDR(cidr); r == nil {
		return true
	}
	return false
}

// get function for network
func (b *glinesData) Network() net.IPNet {
	return b.IpNet
}

// get function for network converted to string
func (b *glinesData) NetworkStr() string {
	return b.IpNet.String()
}
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

func (g *glineData) SecondsUntilExpiration() int64 {
	return int64(g.ExpireTS() - time.Now().Unix())
}

func (g *glineData) HoursSinceLastMod() int64 {
	// If the gline expires in 1 hour and 1 second, this function will return 2
	return int64(math.Ceil(float64((time.Now().Unix() - g.LastModTS())) / 3600.0))
}

func (g *glineData) IsGlineActive() bool {
	return g.active && (g.ExpireTS() > int64(time.Now().Unix()))
}

// Updates glineData.
// If expireTS=0, expireTS value is not modified.
// If reason == nil, reason is not modified
func (g *glineData) Update(active *bool, expireTS int64, reason string) {
	g.lastModTS = time.Now().Unix()
	if active != nil {
		g.active = *active
	}
	if reason != "" {
		g.reason = reason
	}
	if expireTS != 0 {
		g.expireTS = expireTS
	}
	//fmt.Println("DEBUG: glineData.Update():", g.mask, active, expireTS)
}

// create customRangerEntry object using net and asn
func newGlinesData(ipNet net.IPNet, glines []*glineData) cidranger.RangerEntry {
	return &glinesData{
		IpNet:  ipNet,
		Glines: glines,
	}
}

// create customRangerEntry object using net and asn
func newGlineData(ipNet net.IPNet, user, mask string, expireTS, lastModTS int64, reason string, active bool) *glineData {
	return &glineData{
		ipNet:     ipNet,
		user:      user,
		mask:      mask,
		lastModTS: lastModTS,
		expireTS:  expireTS,
		active:    active,
		reason:    reason,
	}
}

// Updates existing glineData information based on gline mask.
// Returns true if ip gline mask exists in current glineData struct. False otherwise.
func (s *serverData) AddOrUpdateGline(ipNet net.IPNet, user, mask string, expireTS, lastModTS int64, reason string, active *bool, line string) bool {
	mask_l := strings.Split(mask, "@")
	if len(mask_l) < 2 {
		return false
	}
	ip := mask_l[1]

	var ipnet *net.IPNet
	entries, err := s.Cranger.ContainingNetworks(net.ParseIP(ip))
	if err != nil {
		var err2 error
		ip = AddCidrToIP(ip)
		_, ipnet, err2 = net.ParseCIDR(ip)
		if err2 != nil {
			log.Printf("Debug: net.ParseCIDR(%s) failed. Line: %s\n", ip, line)
			return false
		}
		entries, err = s.Cranger.CoveringOrCoveredNetworks(*ipnet)
	}
	if err != nil {
		log.Fatalf("Debug: serverData.UpdateGline(): ip=%s, error = %s\n", ip, err.Error())
	}

	//log.Printf("Entries for %s:\n", ip)
	for _, glines := range entries {
		// Cast e (cidranger.RangerEntry to struct glinesData
		gd, ok := glines.(*glinesData)
		if !ok {
			log.Fatalln("This shouldn't have happened")
			continue
		}
		if ipnet.Network() == gd.IpNet.Network() {
			for _, entry := range gd.Glines {
				emask := entry.Mask()
				if strings.EqualFold(mask, emask) {
					log.Printf("DEBUG: serverData.UpdateGline(): Update gline mask=%s\n", mask)
					entry.Update(active, expireTS, reason)
					return true
				}
			}
			if active == nil {
				log.Printf("active is nil for a new gline. That is odd. Gline: %s\n", mask)
			}
			// Add new gline, but another gline exists for that IP, but with a differnet user@.
			log.Printf("DEBUG: serverData.UpdateGline(): Add new gline for mask=%s, but at least one other gline exists with another user for that IP.\n", mask)
			gd.Glines = append(gd.Glines, newGlineData(gd.IpNet, user, mask, expireTS, lastModTS, reason, true))
			return true
		}
	}
	// Add new gline
	if active == nil || reason == "" {
		log.Fatalf("(Insert gline bug) active or reason value is nil for this gline: %s\n", mask)
	}
	//s.AddNewGline(newGlineData(*ipnet, user, mask, expireTS, lastModTS, reason, *active))
	newGline := newGlineData(*ipnet, user, mask, expireTS, lastModTS, reason, *active)
	gList := make([]*glineData, 0, 5)
	gList = append(gList, newGline)
	glineDataList := newGlinesData(*ipnet, gList)
	s.Cranger.Insert(glineDataList)
	return true
}

// This method accepts an IP as parameter and returns two lists:
// active glines and expired/deactivated glines.
// An error is returned if the IP is invalid
// Notes:
//
//	If a gline exists on *@1.2.3.0/24, CheckGline("1.2.3.0/31") will return nothing
//	If a gline exists on *@1.2.3.0/24, CheckGline("1.2.0.0/16") will return the gline
func (s *serverData) CheckGline(ip string, exactCidr bool) ([]*glineData, []*glineData, error) {
	var ipnet *net.IPNet
	entries, err := s.Cranger.ContainingNetworks(net.ParseIP(ip))
	if err != nil {
		var err2 error
		ip = AddCidrToIP(ip)
		_, ipnet, err2 = net.ParseCIDR(ip)
		if err2 != nil {
			log.Printf("Debug: net.ParseCIDR(%s) failed\n", ip)
			return nil, nil, err2
		}
		entries, err = s.Cranger.CoveringOrCoveredNetworks(*ipnet)
	}
	if err != nil {
		log.Printf("Debug: serverData.CheckGline(): ip=%s, error = %s\n", ip, err.Error())
	}
	activeGlines := make([]*glineData, 0, len(entries))
	inactiveGlines := make([]*glineData, 0, len(entries))

	//log.Printf("Entries for %s:\n", ip)
	for _, glines := range entries {
		// Cast e (cidranger.RangerEntry to struct glinesData
		entry, ok := glines.(*glinesData)
		if !ok {
			log.Fatalln("This shouldn't have happened")
			continue
		}
		for _, e := range entry.Glines {
			if exactCidr && ipnet.Network() != entry.IpNet.Network() {
				continue
			}
			if e.IsGlineActive() {
				activeGlines = append(activeGlines, e)
			} else {
				inactiveGlines = append(inactiveGlines, e)
			}
		}
	}
	return activeGlines, inactiveGlines, err
}
