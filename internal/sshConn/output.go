package sshConn

type OutputEvent struct {
	JobID    int
	Hostname string
	Line     string
	System   bool
}
