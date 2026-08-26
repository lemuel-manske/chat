# Chat lifecycle sequence

```mermaid
sequenceDiagram
    autonumber

    actor User
    participant Main as main()
    participant Known as knownPeers
    participant Heartbeat as heartbeatLoop goroutine
    participant Server as createServer goroutine
    participant Connector as connectToPeers goroutine
    participant Input as broadcastStdin goroutine
    participant Remote as Remote Peer
    participant Writer as peerWriter goroutine

    User->>Main: Start with YAML configuration
    Main->>Main: parseArgs()
    Main->>Known: initializeKnownPeers(host)
    Known->>Known: Populate peers from configuration

    par Start heartbeat
        Main->>Heartbeat: go heartbeatLoop(host)
        loop Every 3 seconds — BUM BUM BUM
            Heartbeat->>Heartbeat: copyPeers()
            Heartbeat->>Writer: enqueue PING for each peer
            Writer->>Remote: PING frame
        end

    and Start TCP server
        Main->>Server: go createServer(host)
        Server->>Server: net.Listen(host address and port)

        loop Accept incoming connections
            Remote->>Server: TCP connection
            Server->>Server: go handleServerConnection()

            Remote->>Server: HELLO alias:address:port
            Server->>Known: addKnownPeer(remote)
            Server->>Remote: HELLO local peer
            Server->>Remote: Known peers as PEER frames

            alt Remote alias should own outbound connection
                Server-->>Remote: Close temporary bootstrap connection
            else Local side keeps incoming connection
                Server->>Server: persistConn(remote)
                Server->>Writer: Start dedicated peerWriter
            end
        end

    and Connect to known peers
        Main->>Connector: go connectToPeers(host)
        Connector->>Known: Read configured peers

        loop For every known peer
            alt Local alias is alphabetically smaller
                Connector->>Connector: startDialer(peer)
                Connector->>Remote: TCP connection
                Connector->>Remote: HELLO local peer
                Connector->>Remote: Known peers as PEER frames
                Connector->>Connector: persistConn(peer)
                Connector->>Writer: Start dedicated peerWriter
            else Peer owns the permanent outbound connection
                Connector->>Remote: Temporary bootstrap connection
                Connector->>Remote: HELLO and PEER frames
                Remote-->>Connector: HELLO and discovered PEER frames
                Connector->>Known: addKnownPeer(discovered peers)
                Connector-->>Remote: Close bootstrap connection
            end
        end

    and Read terminal input
        Main->>Input: go broadcastStdin(host)

        loop Read user input
            User->>Input: Enter text or command

            alt Regular message
                Input->>Input: Format "[alias] message"
                Input->>Writer: Enqueue for every connected peer
                Writer->>Remote: Framed broadcast message
            else /msg alias message
                Input->>Writer: Enqueue only for destination
                Writer->>Remote: Framed private message
            else /list
                Input-->>User: Display local and connected peers
            else /quit
                Input->>Remote: BYE frame
                Input->>Main: Exit process
            end
        end
    end

    Main->>Main: select — keep process alive

    opt New peer discovered
        Remote->>Server: PEER alias:address:port
        Server->>Known: addKnownPeer(peer)
        Known->>Connector: maybeStartDialer(peer)
        Server->>Writer: Announce peer to connected peers
    end

    opt Peer stops consuming messages
        Writer->>Writer: Outgoing queue reaches limit
        Writer->>Remote: Close slow connection
        Writer->>Known: Remove active connection
        Note over Writer,Remote: Other peer writers remain independent
    end

    alt Graceful peer departure
        Remote->>Server: BYE
        Server->>Known: Remove active connection
        Server-->>User: "Connection closed by peer"
    else Abrupt peer departure
        Remote-xServer: Connection lost or deadline expires
        Server->>Known: Remove active connection
        Server-->>User: "Connection lost with peer"
    end
```

The `par` block represents the four long-running goroutines started after the
initial known-peer list is populated: heartbeat, TCP listener, outbound
connection management, and terminal input.

```mermaid
sequenceDiagram
    participant G as gabriel
    participant F as fabio

    Note over G,F: gabriel conhece fabio

    G->>F: Temporary TCP bootstrap connection
    G->>F: HELLO:gabriel:address:port
    F->>G: HELLO:fabio:address:port
    F->>G: PEER announcements
    G->>F: PEER announcements
    F-->>G: Close bootstrap connection

    Note over F: fabio descobre gabriel<br/>fabio < gabriel

    F->>G: New permanent TCP connection
    F->>G: HELLO:fabio:address:port
    Note over F,G: A 2a conexão permanece aberta
```
