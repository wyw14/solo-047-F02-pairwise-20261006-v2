package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"path/filepath"
)

type config struct {
	Listen string
	Data   string
}

func parseConfig() (config, error) {
	var value config
	flag.StringVar(&value.Listen, "listen", "127.0.0.1:19703", "HTTP listen address")
	flag.StringVar(&value.Data, "data", "./data", "state data directory")
	flag.Parse()
	if value.Listen == "" || value.Data == "" {
		return config{}, errors.New("listen address and data directory are required")
	}
	if _, _, err := net.SplitHostPort(value.Listen); err != nil {
		return config{}, fmt.Errorf("invalid listen address: %w", err)
	}
	absolute, err := filepath.Abs(value.Data)
	if err != nil {
		return config{}, fmt.Errorf("resolve data directory: %w", err)
	}
	value.Data = absolute
	return value, nil
}
