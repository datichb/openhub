package mcpresolve

// DiagnosticReason tells why an enabled service is not given to the sessions.
type DiagnosticReason string

// ReasonNoToken: the service is enabled but has neither a token_key nor its
// token environment variable. It is left out of the sessions.
const ReasonNoToken DiagnosticReason = "no_token"

// Diagnostic is a service that is enabled and yet left out of the sessions.
// It is a warning, not an error: the other services keep working.
type Diagnostic struct {
	// Service is the name of the service ("linear").
	Service string
	// Reason says why it is left out.
	Reason DiagnosticReason
	// TokenEnv is the variable that would have supplied the token ("" when
	// the service has none).
	TokenEnv string
}

// DiagnoseToken tells whether an enabled service is left out of the sessions
// for want of a token: enabled, no token_key (tokenKey is the resolved one:
// project, then hub) and its token variable empty in the environment (getenv).
// It returns nil when the service is not enabled, has a token source, or is
// not a service that needs a token. The rule is the one the session launch
// applies (bricks.MCPServerUsable); a test keeps them the same.
func DiagnoseToken(service string, enabled bool, tokenKey string, getenv func(string) string) *Diagnostic {
	if !enabled || tokenKey != "" {
		return nil
	}
	svc, ok := Lookup(service)
	if !ok || svc.TokenEnv == "" {
		return nil
	}
	if getenv != nil && getenv(svc.TokenEnv) != "" {
		return nil
	}
	return &Diagnostic{Service: svc.Name, Reason: ReasonNoToken, TokenEnv: svc.TokenEnv}
}
