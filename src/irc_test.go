package ircglineapi

import (
	"strings"
	"testing"

	irc "github.com/fluffle/goirc/client"
)

// Make sure you only use one ip address per test case
// If networks are used in the gline mask, make sure they do not overlap
func TestHandleGNOTICE(t *testing.T) {
	config := &Configuration{
		Network:     "undernet",
		Server:      "hidden.undernet.org",
		Channels:    []string{"#burp"},
		Nick:        "GL",
		Ident:       "stupid",
		Name:        "No name",
		ConnectCmds: []string{},
	}
	irccfg := irc.NewConfig(config.Nick)
	irccfg.SSL = false
	//irccfg.SSLConfig = &tls.Config{ServerName: config.server}
	irccfg.Server = config.Server
	irccfg.Me.Ident = config.Ident
	irccfg.Me.Name = config.Name
	irccfg.NewNick = func(n string) string { return n + "^" }
	ircClient := irc.Client(irccfg)

	s := servers.NewServerInfos(ircClient, config)
	s.ServerName = config.Server
	if s.ServerName != config.Server {
		t.Errorf(`s.serverName != config.Server: %s != %s`, s.ServerName, config.Server)
	}
	//s := servers.GetServerInfos(nil)
	cases := []struct {
		snotice       string
		ip            string
		expireTS      int64
		isDeactivated bool
		expectedErr   bool
	}{
		{
			snotice:       ":hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org adding deactivated global GLINE for *@1.1.1.1, expiring at 1669690015: Unknown G-Line",
			ip:            "1.1.1.1",
			expireTS:      1669690015,
			isDeactivated: true,
			expectedErr:   false,
		},
		{
			snotice:       ":hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org adding global GLINE for *@1.1.1.2, expiring at 1669689587: [0] test",
			ip:            "1.1.1.2",
			expireTS:      1669689587,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       ":hidden.undernet.org NOTICE * :*** Notice -- dronescan.undernet.org adding global GLINE for *@171.253.56.186, expiring at 1670191909: AUTO [0] (171.253.56.186) You were identified as a drone. Email abuse@undernet.org for removal. Visit https://www.undernet.org/gline#drone for more information. (P540)",
			ip:            "171.253.56.186",
			expireTS:      1670191909,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       ":hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.1.1.3: globally deactivating G-line",
			ip:            "1.1.1.3",
			expireTS:      0,
			isDeactivated: true,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.1.1.4: globally deactivating G-line; and changing reason to "[0] test2"`,
			ip:            "1.1.1.4",
			expireTS:      0,
			isDeactivated: true,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- uworld.eu.undernet.org modifying global GLINE for ~*@1.1.1.5: globally activating G-line; changing expiration time to 1670260017; and extending record lifetime to 1670260033`,
			ip:            "1.1.1.5",
			expireTS:      1670260017,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- dronescan.undernet.org modifying global GLINE for *@2a01:cb00:8bd9:4700:cd83:55e2:f420:b455: changing expiration time to 1670207809; and extending record lifetime to 1670207809`,
			ip:            "2a01:cb00:8bd9:4700:cd83:55e2:f420:b455",
			expireTS:      1670207809,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.1.1.6: changing expiration time to 1669689583; extending record lifetime to 1669689583; and changing reason to "Unknown G-Line"`,
			ip:            "1.1.1.6",
			expireTS:      1669689583,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.1.1.7/32: changing expiration time to 1669689583; extending record lifetime to 1669689583; and changing reason to "Unknown G-Line"`,
			ip:            "1.1.1.7/32",
			expireTS:      1669689583,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- uworld.eu.undernet.org adding global GLINE for *@2.1.1.0/24, expiring at 1672262160: AUTO Please make sure identd is installed and properly configured on your router and/or firewall to allow connections from the internet on port 113 before you reconnect. Visit https://www.undernet.org/gline#identd for more information.`,
			ip:            "2.1.1.1",
			expireTS:      1672262160,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- uworld.eu.undernet.org adding global GLINE for *@2a04:dd01:3:5d::/64, expiring at 1672262160: AUTO Please make sure identd is installed and properly configured on your router and/or firewall to allow connections from the internet on port 113 before you reconnect. Visit https://www.undernet.org/gline#identd for more information.`,
			ip:            "2a04:dd01:3:5d::6667",
			expireTS:      1672262160,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.1.1.7: globally activating G-line; changing expiration time to 1734297618; extending record lifetime to 1734297618; and changing reason to "[0] :test2"`,
			ip:            "1.1.1.7",
			expireTS:      1734297618,
			isDeactivated: false,
			expectedErr:   false,
		},
		{
			snotice:       `:hidden.undernet.org NOTICE * :*** Notice -- gnu.undernet.org modifying global GLINE for *@1.1.1.8: globally deactivating G-line; changing expiration time to 1734297618; extending record lifetime to 1734297618; and changing reason to "[0] :test2"`,
			ip:            "1.1.1.8",
			expireTS:      1734297618,
			isDeactivated: true,
			expectedErr:   false,
		},
	}
	//want := true
	for _, c := range cases {
		var res *glineData
		w := strings.Split(c.snotice, " ")
		res_err := handleGNOTICE(c.snotice, w, s)
		res_act, res_deact, res_err_checkg := s.CheckGline(c.ip)
		//t.Logf("res_act = %#v", res_act)
		//t.Logf("res_deact = %#v", res_deact)
		if !c.expectedErr {
			if res_err_checkg != nil {
				t.Errorf(`s.CheckGline(%s) returned an error: %s`, c.ip, res_err_checkg.Error())
			}
			if res_err != nil {
				t.Errorf(`handleGNOTICE(%s) returned an error: %s`, c.snotice, res_err.Error())
			}
		}
		if len(res_act) > 1 {
			t.Errorf(`Unit test cases broken: %s matches more than 1 gline mask.`, c.ip)
		} else if len(res_deact) > 1 {
			t.Errorf(`Unit test cases broken: %s matches more than 1 gline mask.`, c.ip)
		} else if len(res_act) > 0 && len(res_deact) > 0 {
			t.Errorf(`Unit test cases broken: %s matches more than 1 gline mask.`, c.ip)
		} else if len(res_act) == 1 {
			res = res_act[0]
		} else if len(res_deact) == 1 {
			res = res_deact[0]
		} else {
			t.Errorf(`IP address %s not found in s.CheckGline()`, c.ip)
			continue
		}
		if c.expireTS != 0 && res.expireTS != c.expireTS {
			t.Errorf(`res.expireTS(%s) = %v. Want %v`, c.ip, res.expireTS, c.expireTS)
		}
		if res.active != !c.isDeactivated {
			t.Errorf(`res.active(%s) = %t. Want %t`, c.ip, res.active, c.isDeactivated)
		}
	}
}
