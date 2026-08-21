package main

func main() {
	config := parseArgs()

	go heartbeatLoop()

	go createServer(config)
	go connectToPeers(config)
	go broadcastStdin(config)

	select {}
}
