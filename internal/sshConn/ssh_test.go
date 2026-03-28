package sshConn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestDialAddressUsesPort(t *testing.T) {
	host := &Host{Host: "localhost", Port: 2222}
	got := dialAddress(host)
	if got != "localhost:2222" {
		t.Fatalf("unexpected address: %s", got)
	}
}

func TestResolvedAddressUsesPort(t *testing.T) {
	host := ResolvedHost{Host: "example.com", Port: 2200}
	got := resolvedAddress(host)
	if got != "example.com:2200" {
		t.Fatalf("unexpected address: %s", got)
	}
}

func TestHostKeyCallbackWithViperKnownHosts(t *testing.T) {
	dir := t.TempDir()
	khPath := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(khPath, []byte{}, 0o600); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	viper.Set("known_hosts", khPath)
	t.Cleanup(func() { viper.Set("known_hosts", "") })

	cb := hostKeyCallback()
	if cb == nil {
		t.Fatal("expected non-nil callback with valid known_hosts via viper")
	}
}

func TestHostKeyCallbackFallbackInsecure(t *testing.T) {
	viper.Set("known_hosts", "")

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// No .ssh/known_hosts in tmpHome, so it should fall back to insecure
	cb := hostKeyCallback()
	if cb == nil {
		t.Fatal("expected non-nil insecure callback")
	}
}

func TestHostKeyCallbackFallbackToSSHDir(t *testing.T) {
	viper.Set("known_hosts", "")

	tmpHome := t.TempDir()
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("failed to create .ssh dir: %v", err)
	}
	khPath := filepath.Join(sshDir, "known_hosts")
	if err := os.WriteFile(khPath, []byte{}, 0o600); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	cb := hostKeyCallback()
	if cb == nil {
		t.Fatal("expected non-nil callback from ~/.ssh/known_hosts")
	}
}

func TestHostListLifecycleAndState(t *testing.T) {
	hostList := NewHostList()
	if hostList == nil {
		t.Fatal("expected host list")
	}
	if hostList.Len() != 0 {
		t.Fatalf("expected empty list, got %d", hostList.Len())
	}

	h1 := &Host{Hostname: "h1", IsConnected: 1, IsWaiting: 1}
	h2 := &Host{Hostname: "h2", IsConnected: 1, IsWaiting: 0}
	h3 := &Host{Hostname: "h3", IsConnected: 0, IsWaiting: 1}

	hostList.AddHost(h1)
	hostList.AddHost(h2)
	hostList.AddHost(h3)

	if hostList.Len() != 3 {
		t.Fatalf("expected len 3, got %d", hostList.Len())
	}

	hosts := hostList.Hosts()
	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(hosts))
	}

	connected, waiting := hostList.State()
	if connected != 2 || waiting != 1 {
		t.Fatalf("expected connected=2 waiting=1, got connected=%d waiting=%d", connected, waiting)
	}
}
