package common

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"crypto/tls"
)

func newTestSMTPServer(t *testing.T) (string, int, *atomic.Int32, *atomic.Int32, func()) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	certificateTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &certificateTemplate, &certificateTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}})
	if err != nil {
		t.Fatal(err)
	}
	connections := &atomic.Int32{}
	messages := &atomic.Int32{}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			connections.Add(1)
			go serveTestSMTPConnection(conn, messages)
		}
	}()
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_ = listener.Close()
		<-stopped
	}
	return host, port, connections, messages, cleanup
}

func serveTestSMTPConnection(conn net.Conn, messages *atomic.Int32) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeResponse := func(response string) bool {
		if _, err := writer.WriteString(response); err != nil {
			return false
		}
		return writer.Flush() == nil
	}
	if !writeResponse("220 localhost ESMTP ready\r\n") {
		return
	}
	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if inData {
			if line == "." {
				messages.Add(1)
				inData = false
				if !writeResponse("250 queued\r\n") {
					return
				}
			}
			continue
		}
		command := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch command {
		case "EHLO", "HELO":
			if !writeResponse("250-localhost\r\n250 AUTH PLAIN\r\n") {
				return
			}
		case "AUTH":
			if !writeResponse("235 authenticated\r\n") {
				return
			}
		case "MAIL", "RCPT", "RSET", "NOOP":
			if !writeResponse("250 ok\r\n") {
				return
			}
		case "DATA":
			inData = true
			if !writeResponse("354 end with <CRLF>.<CRLF>\r\n") {
				return
			}
		case "QUIT":
			_ = writeResponse("221 bye\r\n")
			return
		default:
			_ = writeResponse(fmt.Sprintf("502 unsupported command %s\r\n", command))
			return
		}
	}
}

func TestSMTPFrequencyLimitBackoff(t *testing.T) {
	oldServer, oldPort := SMTPServer, SMTPPort
	oldAccount, oldToken := SMTPAccount, SMTPToken
	defer func() {
		reusableSMTPConnection.mu.Lock()
		closeReusableSMTPConnectionLocked()
		clearSMTPReconnectBackoffLocked()
		reusableSMTPConnection.mu.Unlock()
		SMTPServer, SMTPPort = oldServer, oldPort
		SMTPAccount, SMTPToken = oldAccount, oldToken
	}()

	SMTPServer, SMTPPort = "127.0.0.1", 1
	SMTPAccount, SMTPToken = "sender@example.com", "test-token"
	limitErr := fmt.Errorf("550 Connection frequency limited")

	reusableSMTPConnection.mu.Lock()
	configKey := currentSMTPConfigKey()
	setSMTPReconnectBackoffLocked(configKey, limitErr)
	remaining := time.Until(reusableSMTPConnection.retryAfter)
	gotErr := smtpReconnectBackoffErrorLocked(configKey)
	reusableSMTPConnection.mu.Unlock()

	if gotErr != limitErr {
		t.Fatalf("expected cached frequency-limit error, got %v", gotErr)
	}
	if remaining < 4*time.Minute+59*time.Second {
		t.Fatalf("expected approximately five minutes of cooldown, got %v", remaining)
	}
	t.Logf("cooldown=%s cached_error=%q", remaining.Round(time.Second), gotErr.Error())
}

func TestSendEmailReusesTLSConnection(t *testing.T) {
	host, port, connections, messages, stopServer := newTestSMTPServer(t)
	defer stopServer()

	oldServer, oldPort := SMTPServer, SMTPPort
	oldAccount, oldToken, oldFrom := SMTPAccount, SMTPToken, SMTPFrom
	oldSSLEnabled, oldForceLogin := SMTPSSLEnabled, SMTPForceAuthLogin
	oldSystemName := SystemName
	defer func() {
		reusableSMTPConnection.mu.Lock()
		closeReusableSMTPConnectionLocked()
		reusableSMTPConnection.mu.Unlock()
		SMTPServer, SMTPPort = oldServer, oldPort
		SMTPAccount, SMTPToken, SMTPFrom = oldAccount, oldToken, oldFrom
		SMTPSSLEnabled, SMTPForceAuthLogin = oldSSLEnabled, oldForceLogin
		SystemName = oldSystemName
	}()

	SMTPServer, SMTPPort = host, port
	SMTPAccount, SMTPToken, SMTPFrom = "sender@example.com", "test-token", "sender@example.com"
	SMTPSSLEnabled, SMTPForceAuthLogin = true, false
	SystemName = "SMTP reuse test"

	if err := SendEmail("first", "receiver@example.com", "first message"); err != nil {
		t.Fatalf("first send failed: %v", err)
	}
	if err := SendEmail("second", "receiver@example.com", "second message"); err != nil {
		t.Fatalf("second send failed: %v", err)
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("expected one TLS connection for two messages, got %d", got)
	}
	if got := messages.Load(); got != 2 {
		t.Fatalf("expected two accepted messages, got %d", got)
	}
	t.Logf("connections=%d messages=%d", connections.Load(), messages.Load())
}
