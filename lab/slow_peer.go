package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	ln, err := net.Listen("tcp", ":9090")
	if err != nil {
		panic(err)
	}

	fmt.Println("slow peer ouvindo em :9090")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}

		fmt.Println("conexão aceita")

		go func(c net.Conn) {
			defer c.Close()

			// propositalmente NÃO lê nada da conexão
			for {
				time.Sleep(time.Minute)
			}
		}(conn)
	}
}
