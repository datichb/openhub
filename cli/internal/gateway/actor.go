package gateway

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// actorMarker separates the person from the oh agent in a Beads actor
// ("Ada Lovelace via oh/developer"): bd records the actor in the history of
// every change (bd history --events).
const actorMarker = " via oh/"

// Actor is the Beads actor of a write: the person the session works for and
// the oh agent that made the change. Unknown agent: "session" (the oh
// session itself) or "subagent".
func Actor(human, agent string) string {
	if agent = cleanActorPart(agent); agent == "" {
		agent = "session"
	}
	if human = cleanActorPart(human); human == "" {
		return "oh/" + agent
	}
	return human + actorMarker + agent
}

// ActorAgent is the oh agent named by an actor written by Actor ("" when
// the actor is not one of oh).
func ActorAgent(actor string) string {
	if _, agent, ok := strings.Cut(actor, actorMarker); ok {
		return agent
	}
	if agent, ok := strings.CutPrefix(actor, "oh/"); ok {
		return agent
	}
	return ""
}

// cleanActorPart keeps an actor part on one printable line.
func cleanActorPart(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// ResolveHuman is the person a session works for, as bd would name them:
// git user.name of the location, else the login name.
func ResolveHuman(ctx context.Context, location string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "git", "-C", location, "config", "user.name").Output(); err == nil {
		if name := cleanActorPart(string(out)); name != "" {
			return name
		}
	}
	for _, k := range []string{"USER", "USERNAME", "LOGNAME"} {
		if v := cleanActorPart(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}
