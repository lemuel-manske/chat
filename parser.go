package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"go.yaml.in/yaml/v4"
)

type Peer struct { // peer
	Alias   string `yaml:"alias"`
	Address string `yaml:"address"`
	Port    string `yaml:"port"`
}

type HostPeer struct { // servidor
	Alias   string `yaml:"alias"`
	Address string `yaml:"address"`
	Port    string `yaml:"port"`
	Peers   []Peer `yaml:"peers"`
}

func (p HostPeer) AddrNPort() string {
	return net.JoinHostPort(p.Address, p.Port)
}

func (p Peer) AddrNPort() string {
	return net.JoinHostPort(p.Address, p.Port)
}

func parseArgs() HostPeer {
	args := os.Args[1:]

	if len(args) < 1 {
		fmt.Println("Usage: chat <config_file>")
		os.Exit(1)
	}

	if len(args) > 1 {
		fmt.Println("Usage: chat <config_file>")
		os.Exit(1)
	}

	yamlFile := args[0]

	yamlData, err := os.ReadFile(yamlFile)

	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
		os.Exit(1)
	}

	config := HostPeer{}

	err = yaml.Unmarshal(yamlData, &config)

	if err != nil {
		fmt.Printf("Error parsing YAML file: %v\n", err)
		os.Exit(1)
	}

	return config
}

func parsePeerMessage(msg, prefix string) (Peer, bool) {
	parts := strings.SplitN(msg, ":", 4)

	if len(parts) != 4 || parts[0] != prefix {
		return Peer{}, false
	}

	alias := strings.TrimSpace(parts[1])
	address := strings.TrimSpace(parts[2])
	port := strings.TrimSpace(parts[3])

	if alias == "" || port == "" {
		return Peer{}, false
	}

	return Peer{
		Alias:   alias,
		Address: address,
		Port:    port,
	}, true
}
