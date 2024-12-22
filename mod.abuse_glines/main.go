package main

import (
	abuse_glines "github.com/hiddn/irc-glines-api/abuse_glines/src"
)

func main() {
	var configFile = "config.json"
	var config abuse_glines.Configuration = abuse_glines.ReadConf(configFile)
	abuse_glines.Api_init(config)
}
