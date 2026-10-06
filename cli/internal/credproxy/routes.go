package credproxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// route is a relayed upstream path, as decoded segments; "{model}" is the
// Bedrock model id segment and "*" any single segment.
type route struct {
	method   string
	segments []string
}

const modelSeg = "{model}"

// routes are the only upstream paths relayed per provider (after the
// /<provider>/ prefix): the inference APIs the agentic tools use. Anything
// else (account, files, fine-tuning, batch…) is refused.
var routes = map[string][]route{
	ProviderBedrock: {
		{http.MethodPost, []string{"model", modelSeg, "converse"}},
		{http.MethodPost, []string{"model", modelSeg, "converse-stream"}},
		{http.MethodPost, []string{"model", modelSeg, "invoke"}},
		{http.MethodPost, []string{"model", modelSeg, "invoke-with-response-stream"}},
		{http.MethodPost, []string{"model", modelSeg, "count-tokens"}},
	},
	ProviderAnthropic: {
		{http.MethodPost, []string{"messages"}},
		{http.MethodPost, []string{"messages", "count_tokens"}},
		{http.MethodGet, []string{"models"}},
		{http.MethodGet, []string{"models", "*"}},
	},
	ProviderOpenAI: {
		{http.MethodPost, []string{"chat", "completions"}},
		{http.MethodPost, []string{"completions"}},
		{http.MethodPost, []string{"responses"}},
		{http.MethodGet, []string{"models"}},
		{http.MethodGet, []string{"models", "*"}},
	},
	ProviderOpenRouter: {
		{http.MethodPost, []string{"chat", "completions"}},
		{http.MethodPost, []string{"completions"}},
		{http.MethodPost, []string{"responses"}},
		{http.MethodGet, []string{"models"}},
	},
}

var (
	errPath  = errors.New("credproxy: path not relayed")
	errModel = errors.New("credproxy: ambiguous model")
)

// parsePath checks the escaped path after /<provider>/ against the routes of
// the provider. Every segment is decoded exactly once; empty, "." and ".."
// segments, and decoded slashes outside the Bedrock model id, are refused.
// It returns the model id carried by the path (Bedrock), if any.
func parsePath(provider, method, escapedRest string) (string, error) {
	if escapedRest == "" {
		return "", errPath
	}
	raw := strings.Split(escapedRest, "/")
	segs := make([]string, len(raw))
	for i, r := range raw {
		dec, err := url.PathUnescape(r)
		if err != nil || dec == "" || dec == "." || dec == ".." || strings.ContainsAny(dec, "\\\x00") {
			return "", errPath
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
			return model, nil
		}
	}
	return "", errPath
}

// bodyModel reads the top-level "model" key of a JSON request body. The key
// must be exact: a duplicate or a case variant ("Model") is refused, since
// the proxy and the provider could read different values. An empty body has
// no model.
func bodyModel(body []byte) (string, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		return "", fmt.Errorf("%w: %v", errModel, err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return "", fmt.Errorf("%w: body is not a JSON object", errModel)
	}
	model, seen := "", false
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("%w: %v", errModel, err)
		}
		key, _ := kt.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return "", fmt.Errorf("%w: %v", errModel, err)
		}
		if !strings.EqualFold(key, "model") {
			continue
		}
		if key != "model" || seen {
			return "", fmt.Errorf("%w: duplicate or case-variant %q key", errModel, key)
		}
		seen = true
		if err := json.Unmarshal(raw, &model); err != nil {
			return "", fmt.Errorf("%w: model is not a string", errModel)
		}
	}
	return model, nil
}
