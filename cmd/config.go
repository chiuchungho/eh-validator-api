package main

import (
	"errors"
	"fmt"

	"github.com/namsral/flag"
)

type config struct {
	nodeEndpoint   string
	relayEndpoints string
	listenAddr     string
	logLevel       string
}

func bindFlags(fs *flag.FlagSet) *config {
	var conf config

	fs.StringVar(&conf.nodeEndpoint,
		"node-endpoint",
		"",
		"ethereum quicknode url")
	fs.StringVar(&conf.relayEndpoints,
		"relays-endpoint",
		"",
		"list of relay endpoints, delimiter with a space, e.g. `https://boost-relay.flashbots.net https://bloxroute.ethical.blxrbdn.com`")
	fs.StringVar(&conf.logLevel,
		"log-level",
		"DEBUG",
		"log level for this app. e.g. DEBUG, INFO")
	fs.StringVar(&conf.listenAddr,
		"listen-addr",
		"",
		"api server listening address")
	return &conf
}

func (c *config) validate() error {
	if c.nodeEndpoint == "" {
		return errors.New("missing param ode-endpoint")
	}

	if c.relayEndpoints == "" {
		return errors.New("missing param relays-endpoints")
	}

	if c.logLevel != "" &&
		c.logLevel != "DEBUG" &&
		c.logLevel != "INFO" &&
		c.logLevel != "WARN" &&
		c.logLevel != "ERROR" {
		return fmt.Errorf(
			`log-level param %s is not allowed. 
			Allowed param: DEBUG, INFO, WARN, ERROR. Default: DEBUG`,
			c.logLevel)
	}

	if c.listenAddr == "" {
		return errors.New("missing param listen-addr")
	}
	return nil
}
