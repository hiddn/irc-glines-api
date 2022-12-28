package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	Channels    []string
	Nick        string
	Ident       string
	Name        string
	ConnectCmds []string
}

type serverData struct {
	conn                 *irc.Conn
	config               *Configuration
	serverName           string
	networkName          string
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
		if strings.EqualFold(srv.networkName, network) {
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

	c.HandleFunc(irc.CONNECTED, handleConnect)
	// And a signal on disconnect
	quit := make(chan bool)
	c.HandleFunc(irc.DISCONNECTED,
		func(conn *irc.Conn, line *irc.Line) { quit <- true })
	c.HandleFunc(irc.PRIVMSG, handlePRIVMSG)
	c.HandleFunc(irc.NOTICE, handleNOTICE)

	c.HandleFunc("001", handle001)
	c.HandleFunc("280", handleGline280)

	// Tell client to connect.
	if err := c.Connect(); err != nil {
		log.Printf("Connection error: %s\n", err.Error())
	}

	// Wait for disconnect
	go start_api()
	<-quit
}

func handlePRIVMSG(conn *irc.Conn, tline *irc.Line) {
	s := servers.GetServerInfos(conn)
	line := strings.Trim(tline.Raw, "\n")
	line = strings.Trim(line, "\r")
	w := strings.Fields(line)
	if len(w) < 4 {
		return
	}
	if w[2][0] == '#' && strings.EqualFold(w[3], ":!g") {
		if len(w) < 5 {
			str := fmt.Sprintf("PRIVMSG %s :Syntax: !g <IP>", w[2])
			s.conn.Raw(str)
			return
		}
		if glines, exp_glines, err := s.CheckGline(w[4]); err == nil {
			str_slices := make([]string, 0, len(glines))
			for _, entry := range glines {
				mask := entry.Mask()
				tmpStr := fmt.Sprintf("%s (expires in <%d hours): %s", mask, entry.HoursUntilExpiration(), entry.reason)
				str_slices = append(str_slices, tmpStr)
				s.conn.Raw(tmpStr)
			}
			for _, entry := range exp_glines {
				mask := entry.Mask()
				tmpStr := fmt.Sprintf("EXPIRED: %s (expired <%d hours ago, lastmod %d hours ago): %s", mask, -entry.HoursUntilExpiration()+1, entry.HoursSinceLastMod(), entry.reason)
				str_slices = append(str_slices, tmpStr)
				s.conn.Raw(tmpStr)
			}
			if len(str_slices) > 0 {
				ret := strings.Join(str_slices, ",  ")
				s.Msg(w[2], ret)
			}
		}
	}
}

func (s *serverData) Msg(dst, msg string) {
	str := fmt.Sprintf("PRIVMSG %s :%s", dst, msg)
	s.conn.Raw(str)
}

func (s *serverData) MsgMainChan(msg string) {
	if !s.conn.Connected() {
		return
	}
	firstchannel := strings.Split(s.config.Channels[0], " ")
	str := fmt.Sprintf("PRIVMSG %s :%s", firstchannel, msg)
	s.conn.Raw(str)
}

func handleConnect(conn *irc.Conn, line *irc.Line) {
	var cfg *Configuration
	s := servers.GetServerInfos(conn)
	cfg = s.config
	for _, cmd := range cfg.ConnectCmds {
		conn.Raw(cmd)
	}
	modeStr := fmt.Sprintf("mode %s +s +33280", conn.Me().Nick)
	conn.Raw(modeStr)
	for _, c := range cfg.Channels {
		conn.Join(c)
	}
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
	reason := strings.Join(w[9:], " ")[1:]
	var active bool
	if w[8] == "-" {
		// Gline is deactivated
		active = false
	} else {
		active = true
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
	ip = AddCidrToIP(ip)
	if _, ip_net, err := net.ParseCIDR(ip); err == nil {
		// cidr is valid
		s.cranger.Insert(newGlineData(*ip_net, user, mask, expireTS, lastModTS, reason, active))
	} else {
		log.Println("Invalid IP/CIDR for mask:", mask)
	}
	//fmt.Println("280:", w[3], expireTS)
}

func handleNOTICE(conn *irc.Conn, line *irc.Line) {
	log.Println(line.Raw)
	s := servers.GetServerInfos(conn)
	w := strings.Split(line.Raw, " ")
	handleGNOTICE(line.Raw, w, s)
}
func handleGNOTICE(line string, w []string, s *serverData) error {
	var err error
	var mask string
	var active bool
	var expireTS, lastModTS int64
	var expireTSstr string
	var retErr error = nil
	expireTSstr = "0"
	reason := "Unknown gline reason"

	if len(w) < 15 {
		return nil
	}
	if w[0][1:] != s.serverName {
		return nil
	}
	if w[2] != "*" {
		return nil
	}
	if w[8] == "deactivated" && w[9] == "global" && w[10] == "GLINE" {
		//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org adding deactivated global GLINE for *@1.1.1.1, expiring at 1669690015: Unknown G-Line
		active = false
		mask = w[12]
		mask = RemoveLastChar(mask)
		if len(w) >= 16 {
			expireTSstr = w[15]
			expireTSstr = RemoveLastChar(expireTSstr)
		} else {
			out := fmt.Sprintf("Parse error: %s", line)
			s.MsgMainChan(out)
			retErr = errors.New(out)
		}
	} else if w[8] != "global" && w[9] != "GLINE" {
		// All the following conditions are for global glines. If they are not, return now.
		return nil
	} else if w[7] == "adding" {
		//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org adding global GLINE for *@1.1.1.1, expiring at 1669689587: [0] test
		// :h27.eu.undernet.org NOTICE * :*** Notice -- dronescan.undernet.org adding global GLINE for *@171.253.56.186, expiring at 1670191909: AUTO [0] (171.253.56.186) You were identified as a drone. Email abuse@undernet.org for removal. Visit https://www.undernet.org/gline#drone for more information. (P540)
		active = true
		mask = w[11]
		mask = RemoveLastChar(mask)
		if len(w) > 15 {
			expireTSstr = w[14]
			expireTSstr = RemoveLastChar(expireTSstr)
			reason = strings.Join(w[15:], " ")
		} else {
			out := fmt.Sprintf("Parse error: %s", line)
			s.MsgMainChan(out)
			retErr = errors.New(out)
		}
		log.Println("DEBUG:", mask, expireTSstr)
	} else if w[7] == "modifying" {
		if w[13] == "deactivating" {
			//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.2.3.4: globally deactivating G-line
			//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.1.1.1: globally deactivating G-line; and changing reason to "[0] test2"
			active = false
			mask = w[11]
			mask = RemoveLastChar(mask)
			expireTSstr = "0"
		} else if w[13] == "activating" {
			//  :h27.eu.undernet.org NOTICE * :*** Notice -- uworld.eu.undernet.org modifying global GLINE for ~*@141.94.71.155: globally activating G-line; changing expiration time to 1670260017; and extending record lifetime to 1670260033
			active = true
			mask = w[11]
			mask = RemoveLastChar(mask)
			if len(w) > 19 {
				expireTSstr = w[19]
				expireTSstr = RemoveLastChar(expireTSstr)
			} else {
				out := fmt.Sprintf("Parse error: %s", line)
				s.MsgMainChan(out)
				retErr = errors.New(out)
			}
		} else if w[13] == "expiration" {
			//  :h27.eu.undernet.org NOTICE * :*** Notice -- dronescan.undernet.org modifying global GLINE for *@2a01:cb00:8bd9:4700:cd83:55e2:f420:b455: changing expiration time to 1670207809; and extending record lifetime to 1670207809
			//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.2.3.4: changing expiration time to 1669689583; extending record lifetime to 1669689583; and changing reason to "Unknown G-Line"
			active = true
			mask = w[11]
			mask = RemoveLastChar(mask)
			if len(w) > 16 {
				expireTSstr = w[16]
				expireTSstr = RemoveLastChar(expireTSstr)
			} else {
				out := fmt.Sprintf("Parse error: %s", line)
				s.MsgMainChan(out)
				retErr = errors.New(out)
			}
			//TODO: send "GLINE <mask>" to server, as it is impossible from the message to know from this message if the gline is active or not. The expiration time will be in the future, even if the gline is being deactivated. I have to make sure that I also adapt handeGline280() to be able to update the info instead of just insert.
		} else {
			out := fmt.Sprintf("Uncaught gline message message: %s", line)
			retErr = errors.New(out)
			return retErr
		}
	}
	mask_l := strings.Split(mask, "@")
	if len(mask_l) < 2 {
		return nil
	}
	user := mask_l[0]
	ip := mask_l[1]
	expireTS, err = strconv.ParseInt(expireTSstr, 10, 64)
	if err != nil {
		log.Fatal("expireTS provided is not an int. String:", line)
	}
	if !s.UpdateGline(mask, active, expireTS) {
		ip := AddCidrToIP(ip)
		if _, ip_net, err := net.ParseCIDR(ip); err == nil {
			lastModTS = time.Now().Unix()
			//fmt.Printf("Adding new gline infos for %s\n", ip)
			s.cranger.Insert(newGlineData(*ip_net, user, mask, expireTS, lastModTS, reason, active))
		} else {
			out := fmt.Sprintf("net.ParseCIDR(%s) error: %s", ip, line)
			s.MsgMainChan(out)
			retErr = errors.New(out)
		}
	}
	return retErr
}

func handle001(conn *irc.Conn, line *irc.Line) {
	s := servers.GetServerInfos(conn)
	w := strings.Split(line.Raw, " ")
	s.serverName = w[0][1:]
	if len(w) >= 6 {
		s.networkName = w[6]
	}
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

// Returns ip/32 if ipv4 address provided without cidr
// Returns ip/128 if ipv6 address provided without cidr
// Returns ip unchanged if char '/' is present in string ip
func AddCidrToIP(ip string) string {
	if strings.Contains(ip, "/") {
		return ip
	}
	if tmpIP := net.ParseIP(ip); tmpIP != nil {
		if version := tmpIP.To4(); version != nil {
			ip += "/32"
		} else {
			ip += "/128"
		}
	}
	return ip
}

func StripCidrFromIP(ip string) string {
	s := strings.Split(ip, "/")
	if len(s) > 1 {
		return s[0]
	}
	return ip
}

func RemoveLastChar(w string) string {
	return w[:len(w)-1]
}

func GetFileNameFromFullPath(f string) string {
	s := strings.Split(f, "/")
	return s[len(f)-1]
}
