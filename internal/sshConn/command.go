package sshConn

type CommandKind int

const (
	CommandKindRun CommandKind = iota
	CommandKindControl
)

type CommandRequest struct {
	JobID       int
	Command     string
	Kind        CommandKind
	ControlByte byte
}
