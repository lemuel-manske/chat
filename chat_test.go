package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const e2eTimeout = 8 * time.Second

type testPeerProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	output chan string
}

func getFreePort(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	defer ln.Close()

	return fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)
}

func writeConfig(
	t *testing.T,
	dir string,
	alias string,
	port string,
	peerAlias string,
	peerPort string,
) string {
	t.Helper()

	path := filepath.Join(dir, alias+".yaml")

	content := fmt.Sprintf(
		"alias: %s\nport: %s\npeers:\n  - alias: %s\n    port: %s\n",
		alias,
		port,
		peerAlias,
		peerPort,
	)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config for %s: %v", alias, err)
	}

	return path
}

func startPeer(t *testing.T, binary string, config string) *testPeerProcess {
	t.Helper()

	cmd := exec.Command(binary, config)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start peer: %v", err)
	}

	p := &testPeerProcess{
		cmd:    cmd,
		stdin:  stdin,
		output: make(chan string, 100),
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			p.output <- scanner.Text()
		}
		close(p.output)
	}()

	t.Cleanup(func() {
		_ = p.stdin.Close()

		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}

		_ = p.cmd.Wait()
	})

	return p
}

func waitForOutput(
	t *testing.T,
	peer *testPeerProcess,
	expected string,
	timeout time.Duration,
) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var seen []string

	for {
		select {
		case line, ok := <-peer.output:
			if !ok {
				t.Fatalf(
					"process ended before output %q was found; saw: %q",
					expected,
					seen,
				)
			}

			seen = append(seen, line)

			if strings.Contains(line, expected) {
				return
			}

		case <-timer.C:
			t.Fatalf(
				"timeout waiting for output %q; saw: %q",
				expected,
				seen,
			)
		}
	}
}

func sendLine(t *testing.T, peer *testPeerProcess, text string) {
	t.Helper()

	if _, err := fmt.Fprintln(peer.stdin, text); err != nil {
		t.Fatalf("failed to send input %q: %v", text, err)
	}
}

func TestChatBidirectionalE2E(t *testing.T) {
	tmp := t.TempDir()

	binary := filepath.Join(tmp, "chat")

	build := exec.Command("go", "build", "-o", binary, ".")
	buildOutput, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"failed to build chat binary: %v\n%s",
			err,
			string(buildOutput),
		)
	}

	kauePort := getFreePort(t)
	gabrielPort := getFreePort(t)

	kaueConfig := writeConfig(
		t,
		tmp,
		"kaue",
		kauePort,
		"gabriel",
		gabrielPort,
	)

	gabrielConfig := writeConfig(
		t,
		tmp,
		"gabriel",
		gabrielPort,
		"kaue",
		kauePort,
	)

	// "kaue" vem depois de "gabriel" alfabeticamente,
	// então pela política atual ele apenas espera o Dial de gabriel.
	kaue := startPeer(t, binary, kaueConfig)

	// Dá ao listener do Kaue uma pequena janela para subir.
	time.Sleep(300 * time.Millisecond)

	gabriel := startPeer(t, binary, gabrielConfig)

	waitForOutput(
		t,
		kaue,
		"Connection accepted from gabriel",
		e2eTimeout,
	)

	waitForOutput(
		t,
		gabriel,
		"Connection established with kaue",
		e2eTimeout,
	)

	sendLine(t, gabriel, "ola kaue")

	waitForOutput(
		t,
		kaue,
		"[gabriel] ola kaue",
		e2eTimeout,
	)

	sendLine(t, kaue, "ola gabriel")

	waitForOutput(
		t,
		gabriel,
		"[kaue] ola gabriel",
		e2eTimeout,
	)
}
