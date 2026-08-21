package main

import (
	"fmt"
	"net"
	"time"
)

/*
 * Um slow peer é um peer que não lê nada da conexão, causando lentidão na rede,
 * para simular um peer mais lento.
 **/

const port = 9090

func main() {
	ln, err := net.Listen("tcp", ":"+fmt.Sprint(port))

	if err != nil {
		panic(err)
	}

	fmt.Printf("slow peer ouvindo em :%d\n", port)

	for {
		conn, err := ln.Accept()

		if err != nil {
			continue
		}

		fmt.Println("conexão aceita")

		go sleeps(conn)
	}
}

func sleeps(c net.Conn) {
	defer c.Close()

	for {
		time.Sleep(time.Minute) // propositalmente NÃO lê nada da conexão
	}
}
