package adsrv

/*
 * types.go
 * JSON message types
 * By J. Stuart McMurray
 * Created 20260808
 * Last Modified 20260809
 */

// ConnType is used in ConnRequest to indicate the type of connection
// requested.
type ConnType string

const (
	ConnTypeUnspecified ConnType = ""
	ConnTypeShellStream ConnType = "shell-stream"
)

// ShellStreamDirection indicates the direction of a stream in a
// ConnTypeShellStream ConnRequest.
type ShellStreamDirection string

const (
	ShellStreamDirectionInput  ShellStreamDirection = "input"  /* /i */
	ShellStreamDirectionInOut  ShellStreamDirection = "in/out" /* /io */
	ShellStreamDirectionOutput ShellStreamDirection = "output" /* /o */
)

// ConnRequest is always the first thing sent by an adapter on a connnection.
type ConnRequest struct {
	ConnType ConnType
	Args     any
}

// ConnResponse is sent to the adapter in response to a ConnRequest.  In
// happy situations, it's just an empty object.
type ConnResponse struct {
	// Error explains why a ConnRequest was rejected.  If the ConnRequest
	// was not rejected, Error will be null.
	Error string `json:",omitzero"`
}

// ConnTypeShellStreamArgs contains information about a shell input, output,
// or input/output connection.
type ConnTypeShellStreamArgs struct {
	// Direction indicates the direction of the stream.
	Direction ShellStreamDirection

	// ID helps ensure that input and output streams belong together and
	// helps correlate log messages related to a single shell.
	// For a bidirectional (InOut) stream, ID is only used for logging.
	// ID should be reasonably collision-resistant.
	ID string

	// Tag is presented to the curlrevshell user in square brackets before
	// connection-related status messages and should give the user a good
	// idea of what's connecting, e.g. a hostname or IP address.
	Tag string

	// LogInfo will be added to log messages related to this connection.
	// It may be any legal JSON value or entirely omitted.
	LogInfo any `json:",omitzero"`
}
