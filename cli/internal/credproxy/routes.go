package credproxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// route is a relayed upstream path, as decoded segments; "{model}" is the
// Bedrock model id segment and "*" any single segment.
type route struct {
	method   string
	segments []string
	// query lists the query parameters relayed on this route, with the
	// values accepted for each (nil = any value). Any other parameter is
	// dropped (audit v5, S-28).
	query map[string][]string
}

const modelSeg = "{model}"

// Query parameters the Anthropic routes take: "beta=true" selects the beta
// namespace of the Messages API (the client of opencode adds it), the model
// list is paginated.
var (
	anthropicBetaQuery = map[string][]string{"beta": {"true"}}
	anthropicListQuery = map[string][]string{"limit": nil, "before_id": nil, "after_id": nil}
)

// routes are the only upstream paths relayed per provider (after the
// /<provider>/ prefix): the inference APIs the agentic tools use. Anything
// else (account, files, fine-tuning, batch…) is refused.
var routes = map[string][]route{
	ProviderBedrock: {
		{http.MethodPost, []string{"model", modelSeg, "converse"}, nil},
		{http.MethodPost, []string{"model", modelSeg, "converse-stream"}, nil},
		{http.MethodPost, []string{"model", modelSeg, "invoke"}, nil},
		{http.MethodPost, []string{"model", modelSeg, "invoke-with-response-stream"}, nil},
		{http.MethodPost, []string{"model", modelSeg, "count-tokens"}, nil},
	},
	ProviderAnthropic: {
		{http.MethodPost, []string{"messages"}, anthropicBetaQuery},
		{http.MethodPost, []string{"messages", "count_tokens"}, anthropicBetaQuery},
		{http.MethodGet, []string{"models"}, anthropicListQuery},
		{http.MethodGet, []string{"models", "*"}, nil},
	},
	ProviderOpenAI: {
		{http.MethodPost, []string{"chat", "completions"}, nil},
		{http.MethodPost, []string{"completions"}, nil},
		{http.MethodPost, []string{"responses"}, nil},
		{http.MethodGet, []string{"models"}, nil},
		{http.MethodGet, []string{"models", "*"}, nil},
	},
	ProviderOpenRouter: {
		{http.MethodPost, []string{"chat", "completions"}, nil},
		{http.MethodPost, []string{"completions"}, nil},
		{http.MethodPost, []string{"responses"}, nil},
		{http.MethodGet, []string{"models"}, nil},
	},
}

var (
	errPath    = errors.New("credproxy: path not relayed")
	errModel   = errors.New("credproxy: ambiguous model")
	errRouting = errors.New("credproxy: model routing refused")
)

// parsePath checks the escaped path after /<provider>/ against the routes of
// the provider. Every segment is decoded exactly once; empty, "." and ".."
// segments, and decoded slashes outside the Bedrock model id, are refused.
// It returns the model id carried by the path (Bedrock), if any.
func parsePath(provider, method, escapedRest string) (string, error) {
	_, model, err := matchRoute(provider, method, escapedRest)
	return model, err
}

// matchRoute is parsePath that also returns the route that matched.
func matchRoute(provider, method, escapedRest string) (route, string, error) {
	if escapedRest == "" {
		return route{}, "", errPath
	}
	raw := strings.Split(escapedRest, "/")
	segs := make([]string, len(raw))
	for i, r := range raw {
		dec, err := url.PathUnescape(r)
		if err != nil || dec == "" || dec == "." || dec == ".." || strings.ContainsAny(dec, "\\\x00") {
			return route{}, "", errPath
		}
		segs[i] = dec
	}
	for _, rt := range routes[provider] {
		if rt.method != method || len(rt.segments) != len(segs) {
			continue
		}
		model, ok := "", true
		for i, want := range rt.segments {
			switch want {
			case modelSeg:
				// Model ids and ARNs may hold an escaped "/" (inference
				// profile ARNs), never a second level of escaping.
				if strings.Contains(segs[i], "%") {
					ok = false
				}
				model = segs[i]
			case "*":
				if strings.Contains(segs[i], "/") {
					ok = false
				}
			default:
				if segs[i] != want {
					ok = false
				}
			}
			if !ok {
				break
			}
		}
		if ok {
			return rt, model, nil
		}
	}
	return route{}, "", errPath
}

// routingKeys are the top-level body keys that choose the model (OpenRouter
// adds fallback lists, routes and presets, audit v5 S-22).
var routingKeys = []string{"model", "models", "route", "preset"}

// bodyRouting reads the routing keys of a JSON request body. Each key must
// be exact: a duplicate or a case variant ("Model") is refused, since the
// proxy and the provider could read different values; so is anything after
// the JSON object. An empty body has no routing key.
func bodyRouting(body []byte) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(body)) == 0 {
		return out, nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errModel, err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("%w: body is not a JSON object", errModel)
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errModel, err)
		}
		key, _ := kt.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("%w: %v", errModel, err)
		}
		for _, rk := range routingKeys {
			if !strings.EqualFold(key, rk) {
				continue
			}
			if _, seen := out[rk]; key != rk || seen {
				return nil, fmt.Errorf("%w: duplicate or case-variant %q key", errModel, key)
			}
			out[rk] = raw
		}
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, fmt.Errorf("%w: %v", errModel, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: data after the JSON object", errModel)
	}
	return out, nil
}

// bodyModel reads the top-level "model" key of a JSON request body (see
// bodyRouting).
func bodyModel(body []byte) (string, error) {
	keys, err := bodyRouting(body)
	if err != nil {
		return "", err
	}
	model := ""
	if raw, ok := keys["model"]; ok {
		if err := json.Unmarshal(raw, &model); err != nil {
			return "", fmt.Errorf("%w: model is not a string", errModel)
		}
	}
	return model, nil
}

// checkRouting refuses, for a session limited to some models, the body
// keys of a provider that choose another model than "model" (OpenRouter:
// "route", "preset", a "models" fallback list with a model not allowed, a
// preset in the model id "…@preset/x").
func checkRouting(provider string, patterns []string, body []byte) error {
	if provider != ProviderOpenRouter || len(patterns) == 0 {
		return nil
	}
	keys, err := bodyRouting(body)
	if err != nil {
		return err
	}
	for _, k := range []string{"route", "preset"} {
		if _, ok := keys[k]; ok {
			return fmt.Errorf("%w: %q is not allowed for a session limited to some models", errRouting, k)
		}
	}
	var model string
	if raw, ok := keys["model"]; ok && json.Unmarshal(raw, &model) == nil && strings.Contains(model, "@") {
		return fmt.Errorf("%w: preset in model %q", errRouting, model)
	}
	if raw, ok := keys["models"]; ok {
		var list []string
		if err := json.Unmarshal(raw, &list); err != nil {
			return fmt.Errorf("%w: models is not a list of model ids", errRouting)
		}
		for _, m := range list {
			if strings.Contains(m, "@") || !modelAllowed(patterns, m) {
				return fmt.Errorf("%w: model %q of models not allowed for this session", errRouting, m)
			}
		}
	}
	return nil
}
