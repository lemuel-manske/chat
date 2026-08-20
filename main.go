package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"

	"go.yaml.in/yaml/v4"
)

type Peer struct {
	Port  string `yaml:"port"`
	Alias string `yaml:"alias"`
	Peers []Peer `yaml:"peers"`
}

func (p Peer) FormatPort() string {
	return fmt.Sprintf(":%s", p.Port)
}

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

func handleServerConnections(ln net.Listener, peer Peer) {
	for {
		conn, err := ln.Accept()

		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}

		fmt.Printf("Connection accepted from: %s\n", conn.RemoteAddr().String())

		go handleServerConnection(conn, peer)
	}
}

func handleServerConnection(conn net.Conn, peer Peer) {
	defer conn.Close()

	var buffer = make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)

		if err != nil {
			fmt.Printf("Error reading from connection: %v\n", err)
			break
		}

		fmt.Printf("%s: %s\n", peer.Alias, string(buffer[:n]))
	}
}

func createClient(peer Peer) {
	fmt.Printf("Connecting to peer: %s\n", peer.Alias)

	for {
		conn, err := net.Dial( // cria o cliente TCP
			"tcp",
			peer.FormatPort(),
		)

		if err != nil {
			fmt.Printf(
				"Could not connect to %s on %s: %v\n",
				peer.Alias,
				peer.FormatPort(),
				err,
			)

			time.Sleep(5 * time.Second)
			continue
		}

		handleClientConnection(conn, peer)
	}
}

func handleClientConnection(conn net.Conn, peer Peer) {
	defer conn.Close()

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		text := scanner.Text()

		_, err := conn.Write([]byte(text))

		if err != nil {
			fmt.Printf("Error writing to connection: %v\n", err)
			break
		}
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
