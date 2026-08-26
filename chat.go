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

const messageCmdFormat = messageCmd + "%s %s" // alias + message

const byeMessagePrefix = "BYE"
const handshakeMessagePrefix = "HELLO"
const peerMessagePrefix = "PEER"
const pingMessagePrefix = "PING"

const byeMessageFormat = byeMessagePrefix + ":%s"                   // alias
const handshakeMessageFormat = handshakeMessagePrefix + ":%s:%s:%s" // alias + address + port
const peerMessageFormat = peerMessagePrefix + ":%s:%s:%s"           // alias + address + port
const pingMessageFormat = pingMessagePrefix + ":%s"                 // alias

const messageFormat = "[%s] %s" // alias + message

var peers = make(map[string]net.Conn)
var peersMutex = sync.Mutex{}

var knownPeers = make(map[string]Peer)
var knownPeersMutex = sync.Mutex{}

var dialers = make(map[string]bool)
var dialersMutex = sync.Mutex{}

// helpers

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

func copyKnownPeers() map[string]Peer {
	tempKnownPeers := make(map[string]Peer)

	knownPeersMutex.Lock()
	maps.Copy(tempKnownPeers, knownPeers)
	knownPeersMutex.Unlock()

	return tempKnownPeers
}

func performHandshake(conn net.Conn, host HostPeer) error {
	address := host.Address

	handshake := fmt.Sprintf(
		handshakeMessageFormat,
		host.Alias,
		address,
		host.Port,
	)

	renewWriteDeadline(conn)

	return writeFrame(conn, []byte(handshake))
}

func isHandshake(msg string) bool {
	return strings.HasPrefix(msg, handshakeMessagePrefix+":")
}

func isPeerAnnouncement(msg string) bool {
	return strings.HasPrefix(msg, peerMessagePrefix+":")
}

func isPing(msg string) bool {
	return strings.HasPrefix(msg, pingMessagePrefix+":")
}

func isBye(msg string) bool {
	return strings.HasPrefix(msg, byeMessagePrefix+":")
}

func isCommand(msg string) bool {
	return strings.HasPrefix(msg, "/")
}

func shouldMaintainOutbound(host HostPeer, peer Peer) bool {
	return host.Alias < peer.Alias
}

// main functions

func createServer(host HostPeer) {
	ln, err := net.Listen(
		"tcp",
		host.Addr(),
	)

	if err != nil {
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()

		if err != nil {
			continue
		}

		go handleServerConnection(conn, host)
	}
}

func handleServerConnection(conn net.Conn, host HostPeer) {
	defer conn.Close()

	var alias string

	renewReadDeadline(conn)

	gracefulClose := false

	for {
		renewReadDeadline(conn)

		payload, err := readFrame(conn)

		if err != nil {
			break
		}

		msg := string(payload)

		if isHandshake(msg) {
			remotePeer, ok := parsePeerMessage(
				msg,
				handshakeMessagePrefix,
			)

			if !ok {
				return
			}

			alias = remotePeer.Alias

			addKnownPeer(remotePeer, host)

			if err := performHandshake(conn, host); err != nil {
				return
			}

			sendKnownPeers(conn)

			// para conexões definitivas, o alias menor deve ser quem iniciou.
			if remotePeer.Alias >= host.Alias {
				return
			}

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

		if isPeerAnnouncement(msg) {
			peer, ok := parsePeerMessage(
				msg,
				peerMessagePrefix,
			)

			if ok {
				addKnownPeer(peer, host)
			}

			continue
		}

		fmt.Println(msg)
	}

	if !gracefulClose {
		fmt.Printf("Connection lost with %s\n", alias)
	}

	removeConn(alias)
}

func connectToPeers(host HostPeer) {
	for _, peer := range host.Peers {
		peer := peer

		if shouldMaintainOutbound(host, peer) {
			startDialer(peer, host)
			continue
		}

		go bootstrapPeer(peer, host)
	}
}

func exchangeBootstrapDiscovery(
	conn net.Conn,
	host HostPeer,
) error {
	if err := performHandshake(conn, host); err != nil {
		return err
	}

	sendKnownPeers(conn)

	renewReadDeadline(conn)

	for {
		renewReadDeadline(conn)

		payload, err := readFrame(conn)

		if err != nil {
			return err
		}

		msg := string(payload)

		if isHandshake(msg) {
			remotePeer, ok := parsePeerMessage(
				msg,
				handshakeMessagePrefix,
			)

			if ok {
				addKnownPeer(remotePeer, host)
			}

			continue
		}

		if isPeerAnnouncement(msg) {
			discoveredPeer, ok := parsePeerMessage(
				msg,
				peerMessagePrefix,
			)

			if ok {
				addKnownPeer(discoveredPeer, host)
			}

			continue
		}
	}
}

func bootstrapPeer(peer Peer, host HostPeer) {
	for {
		conn, err := net.DialTimeout(
			"tcp",
			peer.Addr(),
			networkTimeout,
		)

		if err != nil {
			time.Sleep(sleepDuration)
			continue
		}

		err = exchangeBootstrapDiscovery(conn, host)
		conn.Close()

		if err == nil {
			return
		}

		time.Sleep(sleepDuration)
	}
}

func maintainPeerConnection(peer Peer, host HostPeer) {
	for {
		conn, err := net.DialTimeout(
			"tcp",
			peer.Addr(),
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

		sendKnownPeers(conn)

		fmt.Printf("Connection established with %s\n", peer.Alias)

		persistConn(conn, peer.Alias)

		renewReadDeadline(conn)

		gracefulClose := false

		for {
			payload, err := readFrame(conn)

			if err != nil {
				break
			}

			msg := string(payload)

			renewReadDeadline(conn)

			if isHandshake(msg) || isPing(msg) {
				continue
			}

			if isPeerAnnouncement(msg) {
				discoveredPeer, ok := parsePeerMessage(
					msg,
					peerMessagePrefix,
				)

				if ok {
					addKnownPeer(discoveredPeer, host)
				}

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
	reader := bufio.NewReader(os.Stdin)

	for {
		text, err := reader.ReadString('\n')

		if err != nil && len(text) == 0 {
			return
		}

		text = strings.TrimSuffix(text, "\n")
		text = strings.TrimSuffix(text, "\r")

		if isCommand(text) {
			handleCommand(text, host)
		} else {
			broadcastMessage(text, host)
		}

		if err != nil {
			return
		}
	}
}

func broadcastMessage(text string, host HostPeer) {
	tempPeers := copyPeers()

	message := fmt.Sprintf(
		messageFormat,
		host.Alias,
		text,
	)

	for _, conn := range tempPeers {
		renewWriteDeadline(conn)

		if err := writeFrame(
			conn,
			[]byte(message),
		); err != nil {
			conn.Close()
		}
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

			payload := fmt.Sprintf(
				byeMessageFormat,
				host.Alias,
			)

			if err := writeFrame(
				conn,
				[]byte(payload),
			); err != nil {
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

		payload := fmt.Sprintf(
			messageFormat,
			host.Alias,
			message,
		)

		renewWriteDeadline(conn)

		if err := writeFrame(
			conn,
			[]byte(payload),
		); err != nil {
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

			payload := fmt.Sprintf(
				pingMessageFormat,
				host.Alias,
			)

			if err := writeFrame(
				conn,
				[]byte(payload),
			); err != nil {
				conn.Close()
			}
		}

		time.Sleep(pingTimeout)
	}
}

func initializeKnownPeers(host HostPeer) {
	knownPeersMutex.Lock()
	defer knownPeersMutex.Unlock()

	for _, peer := range host.Peers {
		if peer.Alias == "" || peer.Alias == host.Alias {
			continue
		}

		knownPeers[peer.Alias] = peer
	}
}

func addKnownPeer(peer Peer, host HostPeer) bool {
	if peer.Alias == "" ||
		peer.Alias == host.Alias ||
		peer.Port == "" {
		return false
	}

	knownPeersMutex.Lock()

	current, exists := knownPeers[peer.Alias]

	if exists &&
		current.Address == peer.Address &&
		current.Port == peer.Port {
		knownPeersMutex.Unlock()
		return false
	}

	knownPeers[peer.Alias] = peer
	knownPeersMutex.Unlock()

	fmt.Printf(
		"Peer discovered: %s (%s)\n",
		peer.Alias,
		peer.Addr(),
	)

	maybeStartDialer(peer, host)

	go announcePeer(peer)

	return true
}

func announcePeer(peer Peer) {
	tempPeers := copyPeers()

	message := fmt.Sprintf(
		peerMessageFormat,
		peer.Alias,
		peer.Address,
		peer.Port,
	)

	for _, conn := range tempPeers {
		renewWriteDeadline(conn)

		if err := writeFrame(
			conn,
			[]byte(message),
		); err != nil {
			conn.Close()
		}
	}
}

func sendKnownPeers(conn net.Conn) {
	snapshot := copyKnownPeers()

	for _, peer := range snapshot {
		message := fmt.Sprintf(
			peerMessageFormat,
			peer.Alias,
			peer.Address,
			peer.Port,
		)

		renewWriteDeadline(conn)

		if err := writeFrame(
			conn,
			[]byte(message),
		); err != nil {
			return
		}
	}
}

func startDialer(peer Peer, host HostPeer) {
	dialersMutex.Lock()

	if dialers[peer.Alias] {
		dialersMutex.Unlock()
		return
	}

	dialers[peer.Alias] = true
	dialersMutex.Unlock()

	go maintainPeerConnection(peer, host)
}

func maybeStartDialer(peer Peer, host HostPeer) {
	if peer.Alias == host.Alias {
		return
	}

	if !shouldMaintainOutbound(host, peer) {
		return
	}

	startDialer(peer, host)
}
