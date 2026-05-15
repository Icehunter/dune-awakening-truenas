package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/ssh"
)

var (
	globalSSH   *ssh.Client
	globalDB    *pgx.Conn
	globalPodIP string
	globalPodNS string
)

// cmdConnect is the BubbleTea Cmd fired on Init. It dials SSH, discovers the
// DB pod via kubectl, then opens a pgx connection tunnelled through SSH.
func cmdConnect() tea.Msg {
	keyPath := resolveKeyPath()
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return msgConnect{err: fmt.Errorf("read key %s: %w", keyPath, err)}
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return msgConnect{err: fmt.Errorf("parse key: %w", err)}
	}
	cfg := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", sshHost, cfg)
	if err != nil {
		return msgConnect{err: fmt.Errorf("SSH dial: %w", err)}
	}

	sess, err := client.NewSession()
	if err != nil {
		return msgConnect{err: fmt.Errorf("SSH session: %w", err)}
	}
	out, err := sess.CombinedOutput(
		`sudo kubectl get pods -A -o wide 2>/dev/null | grep db-dbdepl-sts | head -1 | awk '{print $1, $7}'`)
	sess.Close()
	if err != nil {
		return msgConnect{err: fmt.Errorf("kubectl: %w", err)}
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return msgConnect{err: fmt.Errorf("db pod not found")}
	}
	globalPodNS = parts[0]
	podIP := parts[1]
	globalSSH = client
	globalPodIP = podIP

	connStr := fmt.Sprintf(
		"host=127.0.0.1 port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbPort, dbUser, dbPass, dbName)
	pgCfg, err := pgx.ParseConfig(connStr)
	if err != nil {
		return msgConnect{err: err}
	}
	pgCfg.LookupFunc = func(_ context.Context, _ string) ([]string, error) {
		return []string{globalPodIP}, nil
	}
	pgCfg.DialFunc = func(_ context.Context, _, _ string) (net.Conn, error) {
		return globalSSH.Dial("tcp", fmt.Sprintf("%s:%d", globalPodIP, dbPort))
	}
	db, err := pgx.ConnectConfig(context.Background(), pgCfg)
	if err != nil {
		return msgConnect{err: fmt.Errorf("DB connect: %w", err)}
	}
	globalDB = db
	return msgConnect{}
}

// sshExec runs a command on the remote VM and returns combined stdout+stderr.
func sshExec(cmd string) (string, error) {
	if globalSSH == nil {
		return "", fmt.Errorf("not connected")
	}
	sess, err := globalSSH.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return strings.TrimSpace(string(out)), err
}

// sshStream opens a remote command and returns a channel that receives one
// line per send, plus a cancel func that closes the session. The caller must
// return listenForLogLine(ch) from Update to keep reading.
func sshStream(cmd string) (<-chan string, func(), error) {
	if globalSSH == nil {
		return nil, func() {}, fmt.Errorf("not connected")
	}
	sess, err := globalSSH.NewSession()
	if err != nil {
		return nil, func() {}, err
	}
	pipe, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		return nil, func() {}, err
	}
	if err := sess.Start(cmd); err != nil {
		sess.Close()
		return nil, func() {}, err
	}
	ch := make(chan string, 256)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			ch <- sc.Text()
		}
		sess.Wait()
	}()
	cancel := func() { sess.Close() }
	return ch, cancel, nil
}
