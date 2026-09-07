package xray

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
)

// This integration test uses two local Xray instances and local echo servers only.
func TestProxyProtocolsIntegration(t *testing.T) {
	if os.Getenv("ROUTEFLUX_RUN_XRAY_INTEGRATION") != "1" {
		t.Skip("set ROUTEFLUX_RUN_XRAY_INTEGRATION=1 to run local Xray protocol tests")
	}
	binary, err := exec.LookPath("xray")
	if err != nil {
		t.Fatal(err)
	}
	upstreamPort := reserveProxyPorts(t, 1)[0]
	upstream := fmt.Sprintf(`{"log":{"loglevel":"info"},"inbounds":[{"listen":"127.0.0.1","port":%d,"protocol":"socks","settings":{"udp":true}}],"outbounds":[{"protocol":"freedom"}]}`, upstreamPort)
	startProxyXray(t, binary, []byte(upstream))
	ports := reserveProxyPorts(t, 2)
	req := backend.ConfigRequest{LogLevel: "info", AllowLAN: true, SOCKSPort: ports[0], HTTPPort: ports[1], Nodes: []domain.Node{{ID: "upstream", Protocol: domain.ProtocolSocks, Address: "127.0.0.1", Port: upstreamPort}}, SelectedNodeID: "upstream"}
	config, err := NewGenerator().Generate(req)
	if err != nil {
		t.Fatal(err)
	}
	startProxyXray(t, binary, config)
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "proxy-ok") }))
	defer target.Close()
	for _, scheme := range []string{"http", "socks5"} {
		t.Run(scheme, func(t *testing.T) {
			port := ports[1]
			if scheme == "socks5" {
				port = ports[0]
			}
			proxyURL, _ := url.Parse(fmt.Sprintf("%s://127.0.0.1:%d", scheme, port))
			// Only the local test server's self-signed certificate is accepted in this fixture.
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			resp, err := client.Get(target.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "proxy-ok" {
				t.Fatalf("body: %q", body)
			}
		})
	}
	t.Run("socks5-udp", func(t *testing.T) {
		echo, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		defer echo.Close()
		done := make(chan error, 1)
		go func() {
			buf := make([]byte, 4096)
			n, addr, err := echo.ReadFromUDP(buf)
			if err == nil {
				_, err = echo.WriteToUDP(buf[:n], addr)
			}
			done <- err
		}()
		defer func() { _ = echo.Close(); <-done }()
		tcp, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ports[0]), 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer tcp.Close()
		if err = tcp.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err = tcp.Write([]byte{5, 1, 0}); err != nil {
			t.Fatal(err)
		}
		hello := make([]byte, 2)
		if _, err = io.ReadFull(tcp, hello); err != nil {
			t.Fatal(err)
		}
		if hello[1] != 0 {
			t.Fatal("SOCKS authentication failed")
		}
		if _, err = tcp.Write([]byte{5, 3, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
			t.Fatal(err)
		}
		reply := make([]byte, 10)
		if _, err = io.ReadFull(tcp, reply); err != nil {
			t.Fatal(err)
		}
		if reply[1] != 0 || reply[3] != 1 {
			t.Fatalf("UDP associate reply: %v", reply)
		}
		relay := &net.UDPAddr{IP: net.IP(reply[4:8]), Port: int(binaryPort(reply[8:10]))}
		if relay.IP.IsUnspecified() {
			relay.IP = net.IPv4(127, 0, 0, 1)
		}
		udp, err := net.DialUDP("udp4", nil, relay)
		if err != nil {
			t.Fatal(err)
		}
		defer udp.Close()
		if err = udp.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		dst := echo.LocalAddr().(*net.UDPAddr)
		packet := []byte{0, 0, 0, 1, 127, 0, 0, 1, byte(dst.Port >> 8), byte(dst.Port)}
		packet = append(packet, []byte("udp-ok")...)
		if _, err = udp.Write(packet); err != nil {
			t.Fatal(err)
		}
		response := make([]byte, 4096)
		n, err := udp.Read(response)
		if err != nil {
			t.Fatal(err)
		}
		if n < 10 || string(response[10:n]) != "udp-ok" {
			t.Fatalf("UDP response: %v", response[:n])
		}
	})
}

func binaryPort(b []byte) uint16 { return binary.BigEndian.Uint16(b) }

func reserveProxyPorts(t *testing.T, count int) []int {
	t.Helper()
	var sockets []net.Listener
	var udpSockets []*net.UDPConn
	var ports []int
	defer func() {
		for _, s := range sockets {
			_ = s.Close()
		}
		for _, s := range udpSockets {
			_ = s.Close()
		}
	}()
	for i := 0; i < count; i++ {
		s, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		sockets = append(sockets, s)
		port := s.Addr().(*net.TCPAddr).Port
		u, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
		if err != nil {
			t.Fatal(err)
		}
		udpSockets = append(udpSockets, u)
		ports = append(ports, port)
	}
	return ports
}

func startProxyXray(t *testing.T, binary string, config []byte) {
	t.Helper()
	if !json.Valid(config) {
		t.Fatal("invalid fixture config")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, config, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binary, "run", "-config", path)
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer
	ready := make(chan struct{})
	scanDone := make(chan struct{})
	var output strings.Builder
	go func() {
		defer close(scanDone)
		scanner := bufio.NewScanner(reader)
		started := false
		for scanner.Scan() {
			line := scanner.Text()
			output.WriteString(line + "\n")
			if !started && strings.Contains(line, "started") {
				close(ready)
				started = true
			}
		}
	}()
	if err := cmd.Start(); err != nil {
		cancel()
		_ = writer.Close()
		<-scanDone
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { err := cmd.Wait(); _ = writer.Close(); exited <- err }()
	t.Cleanup(func() { cancel(); <-scanDone; _ = reader.Close() })
	select {
	case <-ready:
	case err := <-exited:
		<-scanDone
		t.Fatalf("Xray exited: %v\n%s", err, output.String())
	case <-time.After(10 * time.Second):
		cancel()
		<-scanDone
		t.Fatalf("Xray startup timed out\n%s", output.String())
	}
}
