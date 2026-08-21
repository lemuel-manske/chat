package main

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
)

type Peer struct {
	Alias string `yaml:"alias"`
	Port  string `yaml:"port"`
}

type HostPeer struct {
	Port  string `yaml:"port"`
	Alias string `yaml:"alias"`
	Peers []Peer `yaml:"peers"`
}

func (p HostPeer) FormatPort() string {
	return fmt.Sprintf(":%s", p.Port)
}

func (p Peer) FormatPort() string {
	return fmt.Sprintf(":%s", p.Port)
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
