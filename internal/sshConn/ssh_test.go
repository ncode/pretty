package sshConn

import "testing"

func TestDialAddressUsesPort(t *testing.T) {
	host := &Host{Host: "localhost", Port: 2222}
	got := dialAddress(host)
	if got != "localhost:2222" {
		t.Fatalf("unexpected address: %s", got)
	}
}
