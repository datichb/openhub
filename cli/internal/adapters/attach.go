package adapters

// SingleSessionClient is implemented by adapters whose interactive client
// can be restricted to the session it is attached to: no tabs of other
// sessions, nothing remembered across clients (A43: one session = one
// window). base is the environment the client inherits; the returned env is
// added on top of it. Without this capability, AttachCommand is used as is.
type SingleSessionClient interface {
	SingleSessionAttachCommand(h ServerHandle, sessionID string, base []string) (argv []string, env []string)
}
