package mcp

// LogSink receives notifications/message lines from any Server in the Manager.
// nil-safe: the Manager treats a nil LogSink as a no-op.
type LogSink func(serverName, message string)
