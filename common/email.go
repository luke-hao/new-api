package common

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"net/textproto"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	smtpDefaultReconnectCooldown = 30 * time.Second
	smtpFrequencyLimitedCooldown = 5 * time.Minute
)

type smtpConnectionPool struct {
	mu              sync.Mutex
	client          *smtp.Client
	conn            net.Conn
	configKey       string
	failedConfigKey string
	retryAfter      time.Time
	lastError       error
}

var reusableSMTPConnection smtpConnectionPool

func generateMessageID() (string, error) {
	split := strings.Split(SMTPFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := strings.Split(SMTPFrom, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(SMTPAccount) || slices.Contains(EmailLoginAuthServerList, SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	if shouldUseSMTPLoginAuth() {
		return LoginAuth(SMTPAccount, SMTPToken)
	}
	return smtp.PlainAuth("", SMTPAccount, SMTPToken, SMTPServer)
}

func currentSMTPConfigKey() string {
	return fmt.Sprintf("%s:%d|%s|%s|%t|%t", SMTPServer, SMTPPort, SMTPAccount, SMTPToken, SMTPSSLEnabled, SMTPForceAuthLogin)
}

func isSMTPFrequencyLimited(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "connection frequency limited")
}

func smtpReconnectCooldown(err error) time.Duration {
	if isSMTPFrequencyLimited(err) {
		return smtpFrequencyLimitedCooldown
	}
	return smtpDefaultReconnectCooldown
}

func clearSMTPReconnectBackoffLocked() {
	reusableSMTPConnection.failedConfigKey = ""
	reusableSMTPConnection.retryAfter = time.Time{}
	reusableSMTPConnection.lastError = nil
}

func setSMTPReconnectBackoffLocked(configKey string, err error) {
	reusableSMTPConnection.failedConfigKey = configKey
	reusableSMTPConnection.retryAfter = time.Now().Add(smtpReconnectCooldown(err))
	reusableSMTPConnection.lastError = err
}

func smtpReconnectBackoffErrorLocked(configKey string) error {
	if reusableSMTPConnection.failedConfigKey != configKey {
		clearSMTPReconnectBackoffLocked()
		return nil
	}
	if reusableSMTPConnection.lastError != nil && time.Now().Before(reusableSMTPConnection.retryAfter) {
		return reusableSMTPConnection.lastError
	}
	clearSMTPReconnectBackoffLocked()
	return nil
}

func closeReusableSMTPConnectionLocked() {
	if reusableSMTPConnection.conn != nil {
		_ = reusableSMTPConnection.conn.SetDeadline(time.Now().Add(2 * time.Second))
	}
	if reusableSMTPConnection.client != nil {
		_ = reusableSMTPConnection.client.Close()
	} else if reusableSMTPConnection.conn != nil {
		_ = reusableSMTPConnection.conn.Close()
	}
	reusableSMTPConnection.client = nil
	reusableSMTPConnection.conn = nil
	reusableSMTPConnection.configKey = ""
}

func getReusableSMTPClientLocked(auth smtp.Auth) (*smtp.Client, net.Conn, error) {
	configKey := currentSMTPConfigKey()
	if reusableSMTPConnection.client != nil && reusableSMTPConnection.configKey == configKey {
		return reusableSMTPConnection.client, reusableSMTPConnection.conn, nil
	}
	if err := smtpReconnectBackoffErrorLocked(configKey); err != nil {
		return nil, nil, err
	}

	closeReusableSMTPConnectionLocked()
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         SMTPServer,
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", SMTPServer, SMTPPort), tlsConfig)
	if err != nil {
		setSMTPReconnectBackoffLocked(configKey, err)
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	client, err := smtp.NewClient(conn, SMTPServer)
	if err != nil {
		_ = conn.Close()
		setSMTPReconnectBackoffLocked(configKey, err)
		return nil, nil, err
	}
	if err = client.Auth(auth); err != nil {
		_ = client.Close()
		setSMTPReconnectBackoffLocked(configKey, err)
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	clearSMTPReconnectBackoffLocked()
	reusableSMTPConnection.client = client
	reusableSMTPConnection.conn = conn
	reusableSMTPConnection.configKey = configKey
	return client, conn, nil
}

func sendSMTPMessage(client *smtp.Client, receiverEmails []string, mail []byte) (bool, error) {
	if err := client.Mail(SMTPFrom); err != nil {
		return true, err
	}
	for _, receiver := range receiverEmails {
		if err := client.Rcpt(receiver); err != nil {
			return true, err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return true, err
	}
	if _, err = writer.Write(mail); err != nil {
		return false, err
	}
	if err = writer.Close(); err != nil {
		return false, err
	}
	return false, nil
}

func isSMTPConnectionFailure(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var protocolErr *textproto.Error
	if errors.As(err, &protocolErr) {
		return protocolErr.Code == 421
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "closed network connection")
}

func sendEmailWithReusableTLSConnection(auth smtp.Auth, receiverEmails []string, mail []byte) error {
	reusableSMTPConnection.mu.Lock()
	defer reusableSMTPConnection.mu.Unlock()

	for attempt := 0; attempt < 2; attempt++ {
		client, conn, err := getReusableSMTPClientLocked(auth)
		if err != nil {
			return err
		}
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
		retrySafe, err := sendSMTPMessage(client, receiverEmails, mail)
		if err == nil {
			_ = conn.SetDeadline(time.Time{})
			return nil
		}
		configKey := currentSMTPConfigKey()
		closeReusableSMTPConnectionLocked()
		if isSMTPFrequencyLimited(err) {
			setSMTPReconnectBackoffLocked(configKey, err)
			return err
		}
		if attempt == 0 && retrySafe && isSMTPConnectionFailure(err) {
			continue
		}
		if isSMTPConnectionFailure(err) {
			setSMTPReconnectBackoffLocked(configKey, err)
		}
		return err
	}
	return fmt.Errorf("SMTP send failed after reconnect")
}

func SendEmail(subject string, receiver string, content string) error {
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	id, err2 := generateMessageID()
	if err2 != nil {
		return err2
	}
	if SMTPServer == "" && SMTPAccount == "" {
		return fmt.Errorf("SMTP 服务器未配置")
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	mail := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver, SystemName, SMTPFrom, encodedSubject, time.Now().Format(time.RFC1123Z), id, content))
	auth := getSMTPAuth()
	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	receiverEmails := strings.Split(receiver, ";")
	var err error
	if SMTPPort == 465 || SMTPSSLEnabled {
		err = sendEmailWithReusableTLSConnection(auth, receiverEmails, mail)
	} else {
		err = smtp.SendMail(addr, auth, SMTPFrom, receiverEmails, mail)
	}
	if err != nil {
		SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
	}
	return err
}
