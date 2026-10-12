package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActor(t *testing.T) {
	assert.Equal(t, "Ada Lovelace via oh/developer", Actor("Ada Lovelace", "developer"))
	assert.Equal(t, "Ada via oh/session", Actor("Ada", ""))
	assert.Equal(t, "oh/reviewer", Actor("", "reviewer"))
	assert.Equal(t, "Ada via oh/dev", Actor(" Ada\n", "dev\x1b"), "one printable line")

	assert.Equal(t, "developer", ActorAgent("Ada Lovelace via oh/developer"))
	assert.Equal(t, "reviewer", ActorAgent("oh/reviewer"))
	assert.Equal(t, "developer (remote)", ActorAgent(Actor("Ada", "developer (remote)")))
	assert.Empty(t, ActorAgent("Ada"))
}
