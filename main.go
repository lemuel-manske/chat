package main

func main() {
	host := parseArgs()

	initializeKnownPeers(host)

	go heartbeatLoop(host)
	go createServer(host)
	go connectToPeers(host)
	go broadcastStdin(host)

	// virtual thread do JAVA <-> goroutine

	select {}
}
