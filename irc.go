package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	irc "github.com/fluffle/goirc/client"
	"github.com/yl2chen/cidranger"
)

var servers = make(serversType)
var configFile = "config.json"
var config *Configuration

type Configuration struct {
	network     string
	Server      string
	ChannelName string
	Nick        string
	Ident       string
	Name        string
	ConnectCmds []string
}

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

type serverData struct {
	conn                 *irc.Conn
	config               *Configuration
	lastGlineCmdIssuedTS int64
	cranger              cidranger.Ranger
}

type serversType map[*irc.Conn]*serverData

func (s serversType) NewServerInfos(conn *irc.Conn, config *Configuration) *serverData {
	if srv := s.GetServerInfosByNetwork(config.network); srv != nil {
		log.Fatalln("network exists twice in config file: ", config.network)
	}
	newData := &serverData{
		conn:                 conn,
		config:               config,
		lastGlineCmdIssuedTS: 0,
		cranger:              cidranger.NewPCTrieRanger(),
	}
	s[conn] = newData
	return newData
}

func (s serversType) GetServerInfos(conn *irc.Conn) *serverData {
	if data, ok := s[conn]; ok == true {
		return data
	}
	return nil
}

func (s serversType) GetServerInfosByNetwork(network string) *serverData {
	for _, srv := range s {
		if srv.config.network == network {
			return srv
		}
	}
	return nil
}

func main() {
	config := readConf(configFile)
	irccfg := irc.NewConfig(config.Nick)
	irccfg.SSL = false
	//irccfg.SSLConfig = &tls.Config{ServerName: config.server}
	irccfg.Server = config.Server
	irccfg.Me.Ident = config.Ident
	irccfg.Me.Name = config.Name
	irccfg.NewNick = func(n string) string { return n + "^" }
	c := irc.Client(irccfg)
	servers.NewServerInfos(c, &config)

	// Add handlers to do things here!
	// e.g. join a channel on connect.
	c.HandleFunc(irc.CONNECTED, handleConnect)
	//c.HandleFunc(irc.CONNECTED,
	//	func(conn *irc.Conn, line *irc.Line) { conn.Join(channelName) })
	// And a signal on disconnect
	quit := make(chan bool)
	c.HandleFunc(irc.DISCONNECTED,
		func(conn *irc.Conn, line *irc.Line) { quit <- true })
	c.HandleFunc(irc.PRIVMSG, handlePRIVMSG)

	c.HandleFunc("280", handleGline280)

	// With a "simple" client, set Server before calling Connect...
	//c.Config().Server = serverIpPort

	// Tell client to connect.
	if err := c.Connect(); err != nil {
		fmt.Printf("Connection error: %s\n", err.Error())
	}

	/*
			// ... or, use ConnectTo instead.
		if err := c.ConnectTo(serverIpPort); err != nil {
			fmt.Printf("Connection error: %s\n", err.Error())
		}
	*/

	// Wait for disconnect
	<-quit
}

func handlePRIVMSG(conn *irc.Conn, tline *irc.Line) {
	var cfg *Configuration
	s := servers.GetServerInfos(conn)
	cfg = s.config
	line := strings.Trim(tline.Raw, "\n")
	line = strings.Trim(line, "\r")
	w := strings.Fields(line)
	if len(w) < 4 {
		return
	}
	if strings.EqualFold(w[2], cfg.ChannelName) && strings.EqualFold(w[3], ":!checkgline") {
		if len(w) < 5 {
			str := fmt.Sprintf("PRIVMSG %s :Syntax: !checkgline <IP>", w[2])
			s.conn.Raw(str)
			return
		}
		if glines, err := s.checkGline(w[4]); err == nil {
			str_slices := make([]string, 0, len(glines))
			for _, g := range glines {
				entry, ok := g.(*glineData)
				if !ok {
					continue
				}
				mask := entry.Mask()
				expires := entry.ExpireTS()
				var expireHours float64
				expireHours = float64((expires - time.Now().Unix())) / 3600.0
				tmpStr := fmt.Sprintf("%s (expires in <%d hours)", mask, int(math.Ceil(expireHours)))
				str_slices = append(str_slices, tmpStr)
				s.conn.Raw(tmpStr)
			}
			if len(str_slices) > 0 {
				ret := strings.Join(str_slices, ",  ")
				//s.conn.Raw(ret)
				s.MsgChan(ret)
			}
		}
	}
}

func (s *serverData) Msg(dst, msg string) {
	str := fmt.Sprintf("PRIVMSG %s :%s", dst, msg)
	s.conn.Raw(str)
}

func (s *serverData) MsgChan(msg string) {
	str := fmt.Sprintf("PRIVMSG %s :%s", s.config.ChannelName, msg)
	s.conn.Raw(str)
}

func handleConnect(conn *irc.Conn, line *irc.Line) {
	var cfg *Configuration
	s := servers.GetServerInfos(conn)
	cfg = s.config
	//cfg.ConnectCmds = make([]string, 10)
	for _, cmd := range cfg.ConnectCmds {
		conn.Raw(cmd)
	}
	conn.Join(cfg.ChannelName)
	conn.Raw("gline")
}

func handleGline280(conn *irc.Conn, line *irc.Line) {
	// :h27.eu.undernet.org 280 hid *@74.102.24.245 1666617171 1666530771 1666617171 * + :AUTO [0] (74.102.24.245) You were identified as a drone. Email abuse@undernet.org for removal. Visit https://www.undernet.org/gline#drone for more information. (P540)
	var expireTS, lastModTS int64
	var err error

	s := servers.GetServerInfos(conn)
	w := strings.Split(line.Raw, " ")
	mask := w[3]
	mask_l := strings.Split(mask, "@")
	user := mask_l[0]
	ip := mask_l[1]
	if w[8] == "-" {
		// Gline is deactivated
		return
	}
	expireTS, err = strconv.ParseInt(w[4], 10, 64)
	//fmt.Println(ip, mask, expireTS)
	if err != nil {
		log.Fatal("expireTS provided is not an int")
	}
	lastModTS, err = strconv.ParseInt(w[5], 10, 64)
	if err != nil {
		log.Fatal("lastModTS provided is not an int")
	}
	if tmpIP := net.ParseIP(ip); tmpIP != nil {
		if version := tmpIP.To4(); version != nil {
			ip += "/32"
		} else {
			ip += "/128"
		}
	}
	if _, ip_net, err := net.ParseCIDR(ip); err == nil {
		/* cidr is valid */
		s.cranger.Insert(newGlineData(*ip_net, user, mask, expireTS, lastModTS))
	} else {
		log.Println("Invalid IP/CIDR for mask:", mask)
	}
	//fmt.Println("280:", w[3], expireTS)
}

func (s *serverData) checkGline(ip string) ([]cidranger.RangerEntry, error) {
	// request networks containing this IP
	entries, err := s.cranger.ContainingNetworks(net.ParseIP(ip))
	/*if err != nil {
		fmt.Println("ranger.ContainingNetworks()", err.Error())
		os.Exit(1)
	}*/

	//TODO: Remove the lines below, which is there for debug purposes only
	fmt.Printf("Entries for %s:\n", ip)
	for _, e := range entries {

		// Cast e (cidranger.RangerEntry to struct customRangerEntry
		entry, ok := e.(*glineData)
		if !ok {
			log.Fatalln("This shouldn't have happened")
			continue
		}

		// Get network (converted to string by function)
		n := entry.NetworkStr()

		// Get mask
		mask := entry.Mask()

		// Display
		fmt.Println("\t", n, mask)
	}
	return entries, err
}

func readConf(filename string) Configuration {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("Can't open config file:", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		log.Fatal("config parse error:", err.Error())
	}
	return configuration
}
