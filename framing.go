package main

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
)

const maxMessageSize = 16 * 1024 * 1024 // 16 MiB

func writeFrame(conn net.Conn, payload []byte) error {
	if len(payload) > maxMessageSize {
		return errors.New("message too large")
	}

	header := make([]byte, 4)

	binary.BigEndian.PutUint32(header, uint32(len(payload)))

	if err := writeAll(conn, header); err != nil {
		return err
	}

	return writeAll(conn, payload)
}

func readFrame(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)

	// aqui está a regra principal do framing:
	// - identificar tamanho da mensagem
	// - ler o payload de acordo com o tamanho identificado

	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(header)

	if size > maxMessageSize {
		return nil, errors.New("message too large")
	}

	payload := make([]byte, size)

	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)

		if err != nil {
			return err
		}

		data = data[n:]
	}

	return nil
}
