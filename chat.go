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
)

const networkTimeout = 10 * time.Second
const pingTimeout = 3 * time.Second
const sleepDuration = 5 * time.Second

const listCmd = "/list"
const quitCmd = "/quit"
const messageCmd = "/msg "

const messageCmdFormat = messageCmd + "%s %s\n" // alias + message

const pingMessagePrefix = "PING"
const pingMessageFormat = pingMessagePrefix + ":%s\n" // alias

const byeMessagePrefix = "BYE"
const byeMessageFormat = byeMessagePrefix + ":%s\n" // alias

const handshakeMessagePrefix = "HELLO"
const handshakeMessageFormat = handshakeMessagePrefix + ":%s:%s\n" // alias + port

const messageFormat = "[%s] %s\n" // alias + message

var peers = make(map[string]net.Conn)
var peersMutex = sync.Mutex{}

func renewReadDeadline(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(networkTimeout))
}

func renewWriteDeadline(conn net.Conn) {
	conn.SetWriteDeadline(time.Now().Add(networkTimeout))
}

func persistConn(conn net.Conn, alias string) {
	peersMutex.Lock()
	peers[alias] = conn
	peersMutex.Unlock()
}

func removeConn(alias string) {
	peersMutex.Lock()
	delete(peers, alias)
	peersMutex.Unlock()
}

func copyPeers() map[string]net.Conn {
	tempPeers := make(map[string]net.Conn)

	peersMutex.Lock()
	maps.Copy(tempPeers, peers)
	peersMutex.Unlock()

	return tempPeers
}

func shouldSkipConnection(peer Peer, host HostPeer) bool {
	return peer.Alias < host.Alias
}

func performHandshake(conn net.Conn, host HostPeer) error {
	handshake := fmt.Sprintf(
		handshakeMessageFormat,
		host.Alias,
		host.Port,
	)

	conn.SetWriteDeadline(time.Now().Add(networkTimeout))

	_, err := conn.Write([]byte(handshake))

	if err != nil {
		return err
	}

	return nil
}

func isHandshake(msg string) bool {
	return strings.HasPrefix(msg, handshakeMessagePrefix)
}

func isPing(msg string) bool {
	return strings.HasPrefix(msg, pingMessagePrefix)
}

func isBye(msg string) bool {
	return strings.HasPrefix(msg, byeMessagePrefix)
}

func isCommand(msg string) bool {
	return strings.HasPrefix(msg, "/")
}

func createServer(host HostPeer) {
	ln, err := net.Listen(
		"tcp",
		host.FormatPort(),
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

	renewReadDeadline(conn)

	scanner := bufio.NewScanner(conn)

	gracefulClose := false

	for scanner.Scan() {
		msg := scanner.Text()

		renewReadDeadline(conn)

		if isHandshake(msg) {
			msgParts := strings.SplitN(msg, ":", 3)

			alias = msgParts[1]

			persistConn(conn, alias)

			fmt.Printf("Connection accepted from %s\n", alias)

			continue
		}

		if isPing(msg) {
			continue
		}

		if isBye(msg) {
			fmt.Printf("Connection closed by %s\n", alias)

			removeConn(alias)

			gracefulClose = true

			return
		}

		fmt.Println(msg)
	}

	if !gracefulClose {
		fmt.Printf("Connection lost with %s\n", alias)
	}

	removeConn(alias)
}

func connectToPeers(host HostPeer) {
	for _, p := range host.Peers {
		if shouldSkipConnection(p, host) {
			continue
		}

		go maintainPeerConnection(p, host)
	}
}

func maintainPeerConnection(peer Peer, host HostPeer) {
	for {
		conn, err := net.DialTimeout(
			"tcp",
			peer.FormatPort(),
			networkTimeout,
		)

		if err != nil {
			time.Sleep(sleepDuration)

			continue
		}

		err = performHandshake(conn, host)

		if err != nil {
			conn.Close()

			time.Sleep(sleepDuration)

			continue
		}

		fmt.Printf("Connection established with %s\n", peer.Alias)

		persistConn(conn, peer.Alias)

		renewReadDeadline(conn)

		scanner := bufio.NewScanner(conn)

		gracefulClose := false

		for scanner.Scan() {
			msg := scanner.Text()

			renewReadDeadline(conn)

			shouldSkip := isHandshake(msg) || isPing(msg)

			if shouldSkip {
				continue
			}

			if isBye(msg) {
				fmt.Printf("Connection closed by %s\n", peer.Alias)

				removeConn(peer.Alias)

				conn.Close()

				gracefulClose = true

				return
			}

			fmt.Println(msg)
		}

		if !gracefulClose {
			fmt.Printf("Connection lost with %s\n", peer.Alias)
		}

		removeConn(peer.Alias)

		conn.Close()

		time.Sleep(sleepDuration)
	}
}

func broadcastStdin(host HostPeer) {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		text := scanner.Text()

		if isCommand(text) {
			handleCommand(text, host)

			continue
		}

		tempPeers := copyPeers()

		for _, conn := range tempPeers {
			renewWriteDeadline(conn)

			_, err := fmt.Fprintf(
				conn,
				messageFormat,
				host.Alias,
				text,
			)

			if err != nil {
				conn.Close()
			}
		}
	}

	if scanner.Err() != nil {
		os.Exit(1)
	}
}

func handleCommand(msg string, host HostPeer) {
	if msg == listCmd {
		tempPeers := copyPeers()

		for alias := range tempPeers {
			fmt.Println(alias)
		}

		return
	}

	if msg == quitCmd {
		tempPeers := copyPeers()

		for _, conn := range tempPeers {
			renewWriteDeadline(conn)

			_, err := fmt.Fprintf(
				conn,
				byeMessageFormat,
				host.Alias,
			)

			if err != nil {
				conn.Close()
			}
		}

		os.Exit(0)
	}

	if msg == strings.TrimSpace(messageCmd) {
		fmt.Println("Usage: /msg <alias> <message>")
		return
	}

	if strings.HasPrefix(msg, messageCmd) {
		msgParts := strings.SplitN(msg, " ", 3)

		if len(msgParts) < 3 {
			fmt.Println("Usage: /msg <alias> <message>")
			return
		}

		destinatary := msgParts[1]
		message := msgParts[2]

		tempPeers := copyPeers()

		conn, ok := tempPeers[destinatary]

		if !ok {
			fmt.Printf("No connection with %s\n", destinatary)
			return
		}

		renewWriteDeadline(conn)

		_, err := fmt.Fprintf(
			conn,
			messageFormat,
			host.Alias,
			message,
		)

		if err != nil {
			conn.Close()
		}

		return
	}

	fmt.Println("Unknown command")
}

func heartbeatLoop(host HostPeer) {
	for {
		tempPeers := copyPeers()

		for _, conn := range tempPeers {
			renewWriteDeadline(conn)

			_, err := fmt.Fprintf(
				conn,
				pingMessageFormat,
				host.Alias,
			)

			if err != nil {
				conn.Close()
			}
		}

		time.Sleep(pingTimeout)
	}
}
