package main

import (
	ircgline "github.com/hiddn/irc-glines-api"
)

func main() {
	var configFile = "config.json"
	var config ircgline.Configuration

	config = ircgline.ReadConf(configFile)
	s := ircgline.Irc_init(&config)
	s.Connect()

	// Wait for disconnect
	ircgline.Api_init()
	<-s.Quit
}
