package abuse_glines

import (
	"encoding/json"
	"log"
	"os"
)

func ReadConf(filename string) Configuration {
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

type Configuration struct {
	Networks              []string
	ApiKey                string
	Rules                 []rules
	RecaptchaSecretKey    string
	SecretSessionPassword string
	Smtp                  SmtpConfig
	AbuseEmail            string
	FromEmail             string
	TestEmail             string
	URL                   string
	Testmode              bool
}
