package report

import "example.com/compteur/counter"

// L'API publique demandée ne change pas.
var (
	_ func() counter.Counter       = counter.New
	_ func(counter.Counter) string = Render
)
