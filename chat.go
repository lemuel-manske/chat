package main

import (
	"bufio"
	"fmt"
	"maps"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v4"
)

const dialTimeout = 3 * time.Second
const sleepDuration = 5 * time.Second

const handshakePrefix = "HELLO"
const handshakeMessage = handshakePrefix + ":%s:%s\n" // alias + port

const messageFormat = "[%s] %s\n" // alias + message

var peers = make(map[string]net.Conn)
var peersMutex = sync.Mutex{}

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

func createServer(peer HostPeer) {
	ln, err := net.Listen(
		"tcp",
		peer.FormatPort(),
	)

	if err != nil {
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()

		if err != nil {
			continue
		}

		go handleServerConnection(conn)
	}
}

func handleServerConnection(conn net.Conn) {
	defer conn.Close()

	var alias string

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		msg := scanner.Text()

		if strings.HasPrefix(msg, handshakePrefix) {
			msgParts := strings.SplitN(msg, ":", 3)

			alias = msgParts[1]

			peersMutex.Lock()
			peers[alias] = conn
			peersMutex.Unlock()

			fmt.Printf("Connection accepted from %s\n", alias)
		} else {
			fmt.Println(msg)
		}
	}

	if scanner.Err() != nil {
		fmt.Printf("Connection lost with %s\n", alias)
	}

	peersMutex.Lock()
	delete(peers, alias)
	peersMutex.Unlock()
}

func connectToPeers(host HostPeer) {
	for _, p := range host.Peers {
		if shouldSkipConnection(p, host) {
			continue
		}

		go maintainPeerConnection(p, host)
	}
}

func shouldSkipConnection(peer Peer, host HostPeer) bool {
	return peer.Alias < host.Alias
}

func maintainPeerConnection(peer Peer, host HostPeer) {
	for {
		conn, err := net.DialTimeout(
			"tcp",
			peer.FormatPort(),
			dialTimeout,
		)

		if err != nil {
			time.Sleep(sleepDuration)

			continue
		}

		handshake := fmt.Sprintf(
			handshakeMessage,
			host.Alias,
			host.Port,
		)

		conn.Write([]byte(handshake))

		peersMutex.Lock()
		peers[peer.Alias] = conn
		peersMutex.Unlock()

		scanner := bufio.NewScanner(conn)

		for scanner.Scan() {
			msg := scanner.Text()

			if strings.HasPrefix(msg, handshakePrefix) {
				continue
			}

			fmt.Println(msg)
		}

		if scanner.Err() != nil {
			fmt.Printf("Connection lost with %s\n", peer.Alias)
		}

		peersMutex.Lock()
		delete(peers, peer.Alias)
		peersMutex.Unlock()

		conn.Close()

		time.Sleep(sleepDuration)
	}
}

func broadcastStdin(peer HostPeer) {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		text := scanner.Text()

		tempPeers := make(map[string]net.Conn)

		peersMutex.Lock()
		maps.Copy(tempPeers, peers)
		peersMutex.Unlock()

		for _, conn := range tempPeers {
			fmt.Fprintf(
				conn,
				messageFormat,
				peer.Alias,
				text,
			)
		}
	}

	if scanner.Err() != nil {
		os.Exit(1)
	}
}

func main() {
	config := parseArgs()

	go createServer(config)
	go connectToPeers(config)
	go broadcastStdin(config)

	select {}
}
