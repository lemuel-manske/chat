package main

func main() {
	config := parseArgs()

	go heartbeatLoop(config)
	go createServer(config)
	go connectToPeers(config)
	go broadcastStdin(config)

	select {}
}
