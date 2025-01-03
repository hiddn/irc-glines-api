package ircglineapi

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	irc "github.com/fluffle/goirc/client"
	"github.com/hiddn/cidranger"
)

type serversType map[*irc.Conn]*serverData

var servers = make(serversType)

type serverData struct {
	Conn                 *irc.Conn
	Config               *Configuration
	ServerName           string
	NetworkName          string
	LastGlineCmdIssuedTS int64
	Cranger              cidranger.Ranger
	LoggedInToOperServ   bool
	LastLoginAttempt     int64
	Quit                 chan bool
}

func (s serversType) NewServerInfos(conn *irc.Conn, config *Configuration) *serverData {
	if srv := s.GetServerInfosByNetwork(config.Network); srv != nil {
		log.Fatalln("network exists twice in config file: ", config.Network)
	}
	newData := &serverData{
		Conn:                 conn,
		Config:               config,
		LastGlineCmdIssuedTS: 0,
		Cranger:              cidranger.NewPCTrieRanger(),
		LoggedInToOperServ:   false,
		LastLoginAttempt:     0,
	}
	s[conn] = newData
	return newData
}

func (s serversType) GetServerInfos(conn *irc.Conn) *serverData {
	if data, ok := s[conn]; ok {
		return data
	}
	return nil
}

func (s serversType) GetServerInfosByNetwork(network string) *serverData {
	for _, srv := range s {
		if strings.EqualFold(srv.NetworkName, network) {
			return srv
		}
	}
	return nil
}

func Irc_init(config *Configuration) *serverData {
	irccfg := irc.NewConfig(config.Nick)
	irccfg.SSL = false
	//irccfg.SSLConfig = &tls.Config{ServerName: config.server}
	irccfg.Server = config.Server
	irccfg.Me.Ident = config.Ident
	irccfg.Me.Name = config.Name
	irccfg.NewNick = func(n string) string { return n + "^" }
	c := irc.Client(irccfg)
	s := servers.NewServerInfos(c, config)

	c.HandleFunc(irc.CONNECTED, handleConnect)
	// And a signal on disconnect
	s.Quit = make(chan bool)
	c.HandleFunc(irc.DISCONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			fmt.Printf("Disconnected from IRC server. Reconnecting in %d seconds.\n", s.Config.ReconnWaitTime)
			time.Sleep(time.Duration(s.Config.ReconnWaitTime) * time.Second)
			s.Connect()
			s.Quit <- true // Ne fonctionne pas on dirait bien. Probablement à cause de ircgline.Api_init() qui ne se termine pas
		})
	c.HandleFunc(irc.PRIVMSG, handlePRIVMSG)
	c.HandleFunc(irc.NOTICE, handleNOTICE)
	c.HandleFunc(irc.JOIN, handleJOIN)
	c.HandleFunc(irc.QUIT, handleQUIT)

	c.HandleFunc("001", handle001)
	c.HandleFunc("280", handleGline280)
	c.HandleFunc("401", handle401NoSuchNick)

	// Tell client to connect.
	//if err := c.Connect(); err != nil {
	//	log.Printf("Connection error: %s\n", err.Error())
	//}

	//go Api_init()
	// Wait for disconnect
	return s
}

func handlePRIVMSG(conn *irc.Conn, tline *irc.Line) {
	s := servers.GetServerInfos(conn)
	line := strings.Trim(tline.Raw, "\n")
	line = strings.Trim(line, "\r")
	w := strings.Fields(line)
	if len(w) < 4 {
		return
	}
	if w[2][0] == '#' && strings.EqualFold(w[3], ":!die") {
		s.die()
	}
	if w[2][0] == '#' && strings.EqualFold(w[3], ":!g") {
		if len(w) < 5 {
			str := fmt.Sprintf("PRIVMSG %s :Syntax: !g <IP>", w[2])
			s.Conn.Raw(str)
			return
		}
		if glines, exp_glines, err := s.CheckGline(w[4], false); err == nil {
			str_slices := make([]string, 0, len(glines))
			for _, entry := range glines {
				mask := entry.Mask()
				tmpStr := fmt.Sprintf("%s (expires in %s): %s", mask, time.Duration(entry.SecondsUntilExpiration())*time.Second, entry.reason)
				str_slices = append(str_slices, tmpStr)
				s.Conn.Raw(tmpStr)
			}
			for _, entry := range exp_glines {
				mask := entry.Mask()
				tmpStr := fmt.Sprintf("EXPIRED: %s (expired <%d hours ago, lastmod %d hours ago): %s", mask, -entry.HoursUntilExpiration()+1, entry.HoursSinceLastMod(), entry.reason)
				str_slices = append(str_slices, tmpStr)
				s.Conn.Raw(tmpStr)
			}
			if len(str_slices) > 0 {
				//ret := strings.Join(str_slices, ",  ")
				//s.Msg(w[2], ret)
				for i, res := range str_slices {
					ret := fmt.Sprintf("(%d/%d) %s", i+1, len(str_slices), res)
					s.Conn.Privmsg(w[2], ret)
				}
			} else {
				ret := fmt.Sprintf("No match: %s", w[4])
				s.Conn.Privmsg(w[2], ret)
			}
		}
	}
}

func (s *serverData) Connect() {
	s.LoggedInToOperServ = false
	for {
		if err := s.Conn.Connect(); err != nil {
			log.Printf("Connection error: %s\nTrying again in %d seconds\n", err.Error(), s.Config.ReconnWaitTime)
			time.Sleep(time.Duration(s.Config.ReconnWaitTime) * time.Second)
		} else {
			go s.TimerPing()
			break
		}
	}
}

func (s *serverData) TimerPing() {
	for {
		// Code to execute every 5 minutes
		//fmt.Println("Sending PING", time.Now())
		if s.Conn.Connected() {
			s.Conn.Raw("PING :me")
		} else {
			fmt.Println("Not sending PING (Disconnected)")
			//return
		}
		time.Sleep(60 * time.Second)
	}
}

func (s *serverData) MsgMainChan(msg string) {
	if !s.Conn.Connected() {
		return
	}
	firstchannel := strings.Split(s.Config.Channels[0], " ")[0]
	/*
		// Use built-in Privmsg() instead: it splits long messages
		str := fmt.Sprintf("PRIVMSG %s :%s", firstchannel, msg)
		log.Println("->", str)
		s.Conn.Raw(str)
	*/
	if len(firstchannel) > 0 {
		s.Conn.Privmsg(firstchannel, msg)
	}
}

func handleConnect(conn *irc.Conn, line *irc.Line) {
	var cfg *Configuration
	s := servers.GetServerInfos(conn)
	cfg = s.Config
	s.LoggedInToOperServ = true
	for _, cmd := range cfg.ConnectCmds {
		conn.Raw(cmd)
	}
	modeStr := fmt.Sprintf("mode %s +s +33280", conn.Me().Nick)
	conn.Raw(modeStr)
	conn.Raw(cfg.OperServLogin)
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
		//s.Cranger.Insert(newGlineData(*ip_net, user, mask, expireTS, lastModTS, reason, active))
		//s.AddNewGline(newGlineData(*ip_net, user, mask, expireTS, lastModTS, reason, active))
		s.AddOrUpdateGline(*ip_net, user, mask, expireTS, lastModTS, reason, &active, line.Raw)
	} else {
		log.Println("Invalid IP/CIDR for mask:", mask)
	}
	//fmt.Println("280:", w[3], expireTS)
}

func handleJOIN(conn *irc.Conn, line *irc.Line) {
	log.Println(line.Raw)
	s := servers.GetServerInfos(conn)
	w := strings.Split(line.Raw, " ")
	nick := strings.Split(w[0][1:], "!")[0]
	if nick == s.Config.OperServNick {
		s.Conn.Raw(s.Config.OperServLogin)
		s.LoggedInToOperServ = true
	}
	handleGNOTICE(line.Raw, w, s)
}

func handleQUIT(conn *irc.Conn, line *irc.Line) {
	log.Println(line.Raw)
	s := servers.GetServerInfos(conn)
	w := strings.Split(line.Raw, " ")
	nick := strings.Split(w[0][1:], "!")[0]
	if nick == s.Config.OperServNick {
		s.LoggedInToOperServ = false
	}
	handleGNOTICE(line.Raw, w, s)
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
	var active *bool = new(bool)
	var expireTS, lastModTS int64
	var expireTSstr string = "0"
	var retErr error = nil
	var reason string = ""

	if len(w) < 15 {
		return nil
	}
	if w[0][1:] != s.ServerName {
		return nil
	}
	if w[2] != "*" {
		return nil
	}
	if w[8] == "deactivated" && w[9] == "global" && w[10] == "GLINE" {
		//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org adding deactivated global GLINE for *@1.1.1.1, expiring at 1669690015: Unknown G-Line
		*active = false
		mask = w[12]
		mask = RemoveLastChar(mask)
		if len(w) >= 16 {
			expireTSstr = w[15]
			expireTSstr = RemoveLastChar(expireTSstr)
			if len(w) > 16 {
				reason = strings.Join(w[16:], " ")
			}
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
		*active = true
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
		// *** Notice -- gnu.undernet.org modifying global GLINE for *@test: globally activating G-line; changing expiration time to 1734297618; extending record lifetime to 1734297618; and changing reason to "[0] :test2"
		if w[12] == "globally" {
			if w[13] == "activating" {
				*active = true
			} else if w[13] == "deactivating" {
				*active = false
			} else {
				out := fmt.Sprintf("Parse error: %s", line)
				s.MsgMainChan(out)
				retErr = errors.New(out)
			}
		}
		mask = w[11]
		mask = RemoveLastChar(mask)
		re_exp := regexp.MustCompile(`changing expiration time to (\d+)`)
		re_reason := regexp.MustCompile(`changing reason to "(.*)"$`)
		re_active := regexp.MustCompile(`globally (de)?activating G-line`)

		match_exp := re_exp.FindStringSubmatch(line)
		match_reason := re_reason.FindStringSubmatch(line)
		match_active := re_active.FindStringSubmatch(line)
		//match_ := re_.FindStringSubmatch(line)

		if len(match_exp) > 1 {
			expireTSstr = match_exp[1]
		} else {
			expireTSstr = "0"
		}
		if len(match_reason) > 1 {
			reason = match_reason[1]
		} else {
			reason = ""
		}
		if len(match_active) == 0 {
			active = nil
		} else if len(match_active) == 1 {
			*active = true
		} else if len(match_active) == 2 {
			*active = false
		}

		/*
			if w[13] == "deactivating" {
				//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.2.3.4: globally deactivating G-line
				//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.1.1.1: globally deactivating G-line; and changing reason to "[0] test2"
				*active = false
				mask = w[11]
				mask = RemoveLastChar(mask)
				expireTSstr = "0"
			} else if w[13] == "activating" && w[15] == "changing" {
				//  :h27.eu.undernet.org NOTICE * :*** Notice -- uworld.eu.undernet.org modifying global GLINE for ~*@141.94.71.155: globally activating G-line; changing expiration time to 1670260017; and extending record lifetime to 1670260033
				*active = true
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
			} else if w[13] == "activating" && w[16] == "changing" {
				//  :h27.eu.undernet.org NOTICE * :*** Notice -- uworld.eu.undernet.org modifying global GLINE for *@222.124.21.227: globally activating G-line; and changing expiration time to 1700620682
				*active = true
				mask = w[11]
				mask = RemoveLastChar(mask)
				if len(w) > 20 {
					expireTSstr = w[20]
				} else {
					out := fmt.Sprintf("Parse error: %s", line)
					s.MsgMainChan(out)
					retErr = errors.New(out)
				}
			} else if w[13] == "expiration" {
				//  :h27.eu.undernet.org NOTICE * :*** Notice -- dronescan.undernet.org modifying global GLINE for *@2a01:cb00:8bd9:4700:cd83:55e2:f420:b455: changing expiration time to 1670207809; and extending record lifetime to 1670207809
				//<- :hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.2.3.4: changing expiration time to 1669689583; extending record lifetime to 1669689583; and changing reason to "Unknown G-Line"
				*active = true
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
				s.MsgMainChan(out)
				retErr = errors.New(out)
				return retErr
			}
		*/
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
	lastModTS = time.Now().Unix()
	ip = AddCidrToIP(ip)
	if _, ip_net, err := net.ParseCIDR(ip); err == nil {
		s.AddOrUpdateGline(*ip_net, user, mask, expireTS, lastModTS, reason, active, line)
	} else {
		out := fmt.Sprintf("net.ParseCIDR(%s) error: %s", ip, line)
		s.MsgMainChan(out)
		retErr = errors.New(out)
	}
	return retErr
}

func handle401NoSuchNick(conn *irc.Conn, line *irc.Line) {
	s := servers.GetServerInfos(conn)
	log.Printf("No such nick/channel: %s\n", line.Args[1])
	if line.Args[1] == s.Config.OperServNick {
		s.LoggedInToOperServ = false
	}
}

func handle001(conn *irc.Conn, line *irc.Line) {
	s := servers.GetServerInfos(conn)
	w := strings.Split(line.Raw, " ")
	s.ServerName = w[0][1:]
	if len(w) >= 6 {
		s.NetworkName = w[6]
	}
}

func (s *serverData) die() {
	s.Conn.Raw("QUIT :Killed")
	time.Sleep(1 * time.Second)
	os.Exit(0)
}

func (s *serverData) sendCommandToOperServ(cmd string) {
	if s.Config.AutologinIfOperServMissing && !s.LoggedInToOperServ && (time.Now().Unix()-s.LastLoginAttempt) > 120 {
		s.LastLoginAttempt = time.Now().Unix()
		s.Conn.Raw(s.Config.OperServLogin)
	}
	s.Conn.Privmsg(s.Config.OperServNick, cmd)
}
