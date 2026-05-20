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
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/ssh"
)

var (
	globalSSH   *ssh.Client
	globalDB    *pgxpool.Pool
	globalPodIP string
	globalPodNS string
	globalPod   string
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
	// Use jsonpath to extract namespace + podIP directly — avoids awk column
	// miscount when RESTARTS shows "1 (32m ago)" instead of "0".
	out, err := sess.CombinedOutput(
		`sudo kubectl get pods -A -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.status.podIP}{"\n"}{end}' 2>/dev/null | grep db-dbdepl-sts | head -1`)
	sess.Close()
	if err != nil {
		return msgConnect{err: fmt.Errorf("kubectl: %w", err)}
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 3 {
		return msgConnect{err: fmt.Errorf("db pod not found")}
	}
	globalPodNS = parts[0]
	globalPod = parts[1]
	podIP := parts[2]
	globalSSH = client
	globalPodIP = podIP

	pool, err := connectDB(context.Background(), dbUser, dbPass)
	if err != nil && dbUser == "dune" && dbPass == "dune" {
		if postgresPass, passErr := discoverPostgresPassword(client); passErr == nil && postgresPass != "" {
			pool, err = connectDB(context.Background(), "postgres", postgresPass)
		}
	}
	if err != nil {
		client.Close()
		globalSSH = nil
		return msgConnect{err: fmt.Errorf("DB connect: %w", err)}
	}
	globalDB = pool
	return msgConnect{}
}

func connectDB(ctx context.Context, user, pass string) (*pgxpool.Pool, error) {
	connStr := fmt.Sprintf(
		"host=127.0.0.1 port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbPort, user, pass, dbName)
	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf(`SET search_path TO %s, public`, pgx.Identifier{dbSchema}.Sanitize()))
		return err
	}
	poolCfg.ConnConfig.LookupFunc = func(_ context.Context, _ string) ([]string, error) {
		return []string{globalPodIP}, nil
	}
	poolCfg.ConnConfig.DialFunc = func(_ context.Context, _, _ string) (net.Conn, error) {
		return globalSSH.Dial("tcp", fmt.Sprintf("%s:%d", globalPodIP, dbPort))
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	dbUser = user
	dbPass = pass
	return pool, nil
}

func discoverPostgresPassword(client *ssh.Client) (string, error) {
	if globalPodNS == "" || globalPod == "" {
		return "", fmt.Errorf("db pod not discovered")
	}
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	cmd := fmt.Sprintf("sudo kubectl exec -n %s %s -- printenv POSTGRES_PASSWORD", shellQuote(globalPodNS), shellQuote(globalPod))
	out, err := sess.CombinedOutput(cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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
