package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"

	"go.yaml.in/yaml/v4"
)

func readConfigFile() Peer {
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

	config := Peer{}

	err = yaml.Unmarshal(yamlData, &config)

	if err != nil {
		fmt.Printf("Error parsing YAML file: %v\n", err)
		os.Exit(1)
	}

	return config
}

type Peer struct {
	Port  string `yaml:"port"`
	Alias string `yaml:"alias"`
	Peers []Peer `yaml:"peers"`
}

func (p Peer) FormatPort() string {
	return fmt.Sprintf(":%s", p.Port)
}

func createServer(peer Peer) {
	ln, err := net.Listen( // cria o servidor TCP
		"tcp",
		peer.FormatPort(),
	)

	if err != nil {
		os.Exit(1)
	}

	go handleServerConnections(ln, peer)
}

func createClient(peer Peer) {
	fmt.Printf("Connecting to peer: %s\n", peer.Alias)

	for {
		conn, err := net.Dial( // cria o cliente TCP
			"tcp",
			peer.FormatPort(),
		)

		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		go handleClientConnection(conn)
	}
}

func handleServerConnections(ln net.Listener, peer Peer) {
	for {
		conn, err := ln.Accept()

		if err != nil {
			continue
		}

		fmt.Printf("Connection accepted")

		go handleServerConnection(conn)
	}
}

func handleServerConnection(conn net.Conn) {
	defer conn.Close()

	var buffer = make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)

		if err != nil {
			break
		}

		fmt.Printf(string(buffer[:n]))
	}
}

func handleClientConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		conn.Write([]byte(scanner.Text()))
	}
}

func main() {
	config := readConfigFile()

	createServer(config)

	for _, peer := range config.Peers {
		go createClient(peer)
	}

	select {} // manter a gorotina principal em execução
}
