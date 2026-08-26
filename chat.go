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

const (
	networkTimeout = 10 * time.Second
	pingTimeout    = 3 * time.Second
	sleepDuration  = 5 * time.Second
)

const (
	listCmd = "/list"
	quitCmd = "/quit"
	msgCmd  = "/msg " // "msg" ou "msg " para permitir que o alias seja separado do comando
)

const msgCmdFormat = msgCmd + "%s %s" // alias + message

const (
	byeMessagePrefix       = "BYE" // quit
	handshakeMessagePrefix = "HELLO" // handshake
	peerMessagePrefix      = "PEER" // anunciar peers
	pingMessagePrefix      = "PING" // keep-alive
)

const (
	byeMessageFormat       = byeMessagePrefix + ":%s"             // alias
	handshakeMessageFormat = handshakeMessagePrefix + ":%s:%s:%s" // alias + address + port
	peerMessageFormat      = peerMessagePrefix + ":%s:%s:%s"      // alias + address + port
	pingMessageFormat      = pingMessagePrefix + ":%s"            // alias
)

const messageFormat = "[%s] %s" // alias + message

const peerQueueSize = 1024

type PeerConnection struct {
	Conn   net.Conn
	SendCh chan []byte   // canal de envio
	Done   chan struct{} // canal de encerramento

	writeMutex sync.Mutex
}

func (pc *PeerConnection) write(payload []byte) error {
	pc.writeMutex.Lock()
	defer pc.writeMutex.Unlock()

	return writeFrame(pc.Conn, payload)
}

var peers = make(map[string]*PeerConnection)
var peersMutex = sync.Mutex{}

var knownPeers = make(map[string]Peer)
var knownPeersMutex = sync.Mutex{}

var dialers = make(map[string]bool)
var dialersMutex = sync.Mutex{}

func renewReadDeadline(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(networkTimeout))
}

func renewWriteDeadline(conn net.Conn) {
	conn.SetWriteDeadline(time.Now().Add(networkTimeout))
}

func copyPeers() map[string]*PeerConnection {
	peersMutex.Lock()

	defer peersMutex.Unlock()

	copy := make(
		map[string]*PeerConnection,
		len(peers),
	)

	for alias, pc := range peers {
		copy[alias] = pc
	}

	return copy
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

func isPermanentDialer(dialerAlias, listenerAlias string) bool {
	return dialerAlias < listenerAlias // política de desempate por ordem alfabética
}

func shouldMaintainOutbound(host HostPeer, peer Peer) bool {
	// para conexões definitivas, o alias menor deve ser quem iniciou (alfabeticamente)

	return isPermanentDialer(host.Alias, peer.Alias)
}

func persistConn(conn net.Conn, alias string) *PeerConnection {
	pc := &PeerConnection{
		Conn:   conn,
		SendCh: make(chan []byte, peerQueueSize),
		Done:   make(chan struct{}),
	}

	peersMutex.Lock()

	old := peers[alias]
	peers[alias] = pc

	peersMutex.Unlock()

	if old != nil {
		old.Conn.Close()
	}

	go peerWriter(alias, pc)

	return pc
}

func peerWriter(alias string, pc *PeerConnection) {
	defer pc.Conn.Close()

	for {
		select {
		case payload := <-pc.SendCh:
			renewWriteDeadline(pc.Conn)

			if err := pc.write(payload); err != nil {
				removeConnIfCurrent(alias, pc)
				return
			}

		case <-pc.Done:
			return
		}
	}
}

func removeConnIfCurrent(
	alias string,
	pc *PeerConnection,
) {
	peersMutex.Lock()
	defer peersMutex.Unlock()

	current, ok := peers[alias]

	if !ok || current != pc {
		return
	}

	delete(peers, alias)

	select {
	case <-pc.Done:
	default:
		close(pc.Done)
	}
}

// todas as rotinas de envio de mensagens devem passar por aqui,
// para evitar que um peer lento bloqueie o envio para outros peers
func enqueueMessage(
	alias string,
	pc *PeerConnection,
	payload []byte,
) bool {
	select {
	case pc.SendCh <- payload:
		return true

	default:
		fmt.Printf(
			"Peer %s is too slow; disconnecting\n",
			alias,
		)

		removeConnIfCurrent(alias, pc)
		pc.Conn.Close()

		return false
	}
}

func createServer(host HostPeer) {
	ln, err := net.Listen(
		"tcp",
		host.AddrNPort(),
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
	var pc *PeerConnection

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

			if !isPermanentDialer(remotePeer.Alias, host.Alias) {
				return
			}

			pc = persistConn(conn, alias)

			fmt.Printf("Connection accepted from %s\n", alias)

			continue
		}

		if isPing(msg) {
			continue
		}

		if isBye(msg) {
			fmt.Printf("Connection closed by %s\n", alias)

			if pc != nil {
				removeConnIfCurrent(alias, pc)
			}

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

	if pc != nil {
		fmt.Printf(
			"Connection lost with %s\n",
			alias,
		)

		removeConnIfCurrent(alias, pc)
	}
}

func connectToPeers(host HostPeer) {
	for _, peer := range host.Peers {
		// gabriel -> kaue
		// kaue -> gabriel
		// ?
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
			peer.AddrNPort(),
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
			peer.AddrNPort(),
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

		pc := persistConn(conn, peer.Alias)

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

				removeConnIfCurrent(peer.Alias, pc)

				conn.Close()

				gracefulClose = true

				return
			}

			fmt.Println(msg)
		}

		if !gracefulClose {
			fmt.Printf("Connection lost with %s\n", peer.Alias)
		}

		removeConnIfCurrent(peer.Alias, pc)

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

	payload := []byte(message)

	for alias, pc := range tempPeers {
		enqueueMessage(
			alias,
			pc,
			payload,
		)
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

		byeMsg := fmt.Sprintf(
			byeMessageFormat,
			host.Alias,
		)

		payload := []byte(byeMsg)

		for _, pc := range tempPeers {
			renewWriteDeadline(pc.Conn)

			if err := pc.write(payload); err != nil {
				pc.Conn.Close()
			}
		}

		os.Exit(0)
	}

	if msg == strings.TrimSpace(msgCmd) {
		fmt.Println("Usage: /msg <alias> <message>")
		return
	}

	if strings.HasPrefix(msg, msgCmd) {
		msgParts := strings.SplitN(msg, " ", 3)

		if len(msgParts) < 3 {
			fmt.Println("Usage: /msg <alias> <message>")
			return
		}

		destinatary := msgParts[1]
		message := msgParts[2]

		tempPeers := copyPeers()

		pc, ok := tempPeers[destinatary]

		if !ok {
			fmt.Printf("No connection with %s\n", destinatary)

			return
		}

		msg := fmt.Sprintf(
			messageFormat,
			host.Alias,
			message,
		)

		payload := []byte(msg)

		enqueueMessage(
			destinatary,
			pc,
			payload,
		)

		return
	}

	fmt.Println("Unknown command")
}

func heartbeatLoop(host HostPeer) {
	for {
		tempPeers := copyPeers()

		message := fmt.Sprintf(
			pingMessageFormat,
			host.Alias,
		)

		payload := []byte(message)

		for alias, pc := range tempPeers {
			enqueueMessage(
				alias,
				pc,
				payload,
			)
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
	isValid := peer.Alias != "" && peer.Alias != host.Alias && peer.Port != ""

	if !isValid {
		return false
	}

	knownPeersMutex.Lock()

	current, exists := knownPeers[peer.Alias]

	isSame := exists &&
		current.Address == peer.Address &&
		current.Port == peer.Port

	if isSame {
		knownPeersMutex.Unlock()
		return false
	}

	knownPeers[peer.Alias] = peer
	knownPeersMutex.Unlock()

	fmt.Printf(
		"Peer discovered: %s (%s)\n",
		peer.Alias,
		peer.AddrNPort(),
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

	payload := []byte(message)

	for alias, pc := range tempPeers {
		enqueueMessage(
			alias,
			pc,
			payload,
		)
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

		// não enfileirar, pois é uma conexão temporária de bootstrap
		if err := writeFrame(
			conn,
			[]byte(message),
		); err != nil {
			return
		}
	}
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
