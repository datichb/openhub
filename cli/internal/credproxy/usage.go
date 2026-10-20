package credproxy

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"regexp"
	"strconv"
)

// Usage fields across providers:
//   - Bedrock Converse (event-stream frames carry JSON): "inputTokens",
//     "outputTokens", cache "cacheReadInputTokens", "cacheWriteInputTokens"
//     (the input count leaves the cache out)
//   - Bedrock InvokeModel with response stream: event-stream frames whose JSON
//     payload carries the model chunk in base64 ({"bytes": "…"}); Anthropic
//     chunks inside: "input_tokens", "output_tokens", "cache_read_input_tokens",
//     "cache_creation_input_tokens", and the final "amazon-bedrock-invocationMetrics"
//     "inputTokenCount", "outputTokenCount", "cacheReadInputTokenCount",
//     "cacheWriteInputTokenCount"
//   - Anthropic Messages (SSE): "input_tokens", "output_tokens" (message_delta
//     is cumulative), "cache_read_input_tokens", "cache_creation_input_tokens"
//     (the input count leaves the cache out)
//   - OpenAI / OpenRouter: "prompt_tokens", "completion_tokens" (streams carry
//     them only with stream_options.include_usage, added by the proxy); the
//     cache is a part of the prompt: "cached_tokens", "cache_write_tokens"
//     (OpenAI Responses: "input_tokens" with the same details)
var tokenRe = regexp.MustCompile(`"(inputTokens|input_tokens|prompt_tokens|inputTokenCount|` +
	`outputTokens|output_tokens|completion_tokens|outputTokenCount|` +
	`cache_read_input_tokens|cacheReadInputTokens|cacheReadInputTokenCount|` +
	`cache_creation_input_tokens|cacheWriteInputTokens|cacheWriteInputTokenCount|` +
	`cached_tokens|cache_write_tokens)"\s*:\s*(\d+)`)

type tokenKind int

const (
	kindInput tokenKind = iota + 1
	kindOutput
	kindCacheRead  // on top of the input count
	kindCacheWrite // on top of the input count
	kindCachedPart // part of the input count
	kindWrittenPart
)

var tokenKinds = map[string]tokenKind{
	"inputTokens": kindInput, "input_tokens": kindInput, "prompt_tokens": kindInput, "inputTokenCount": kindInput,
	"outputTokens": kindOutput, "output_tokens": kindOutput, "completion_tokens": kindOutput, "outputTokenCount": kindOutput,
	"cache_read_input_tokens": kindCacheRead, "cacheReadInputTokens": kindCacheRead, "cacheReadInputTokenCount": kindCacheRead,
	"cache_creation_input_tokens": kindCacheWrite, "cacheWriteInputTokens": kindCacheWrite, "cacheWriteInputTokenCount": kindCacheWrite,
	"cached_tokens": kindCachedPart, "cache_write_tokens": kindWrittenPart,
}

// terminalRe matches what ends a complete stream: the Anthropic and
// Responses end events, the OpenAI sentinel, the Bedrock final usage.
var terminalRe = regexp.MustCompile(`"type"\s*:\s*"(?:message_stop|response\.completed|response\.incomplete|response\.failed|error)"|\[DONE\]|"totalTokens"|invocationMetrics`)

const scanWindow = 4096

// maxFrame bounds an AWS event-stream frame (the service limit is 16 MiB).
const maxFrame = 16 << 20

// usageReader passes the response through unchanged while scanning it for
// usage counters. Per response, the highest value seen for each counter is
// accounted (streams repeat or accumulate counters). A response cut before
// its end is accounted with what was seen, or an estimate that errs high.
type usageReader struct {
	rc       io.ReadCloser
	g        *grantState
	onReport func(Usage) // accounts the response (Proxy.account, reservations)
	tail     []byte
	seen     map[tokenKind]int64
	reported bool
	// frames decodes AWS event-stream frames with base64 payloads
	// (Bedrock invoke-with-response-stream).
	frames bool
	buf    []byte

	// stream: the body is an event stream, which shows its own end.
	stream   bool
	terminal bool
	eof      bool
	bytes    int64
	// estimate: a cut response is estimated (successful model call);
	// reqBytes and maxOutput bound the estimate (0 = unknown).
	estimate  bool
	reqBytes  int64
	maxOutput int64
}

func (u *usageReader) Read(p []byte) (int, error) {
	n, err := u.rc.Read(p)
	if n > 0 {
		u.bytes += int64(n)
		if u.frames {
			u.scanFrames(p[:n])
		} else {
			u.scan(p[:n])
		}
	}
	if err == io.EOF {
		u.eof = true
		u.report()
	}
	return n, err
}

func (u *usageReader) Close() error {
	u.report()
	return u.rc.Close()
}

func (u *usageReader) scan(chunk []byte) {
	buf := make([]byte, 0, len(u.tail)+len(chunk))
	buf = append(buf, u.tail...)
	buf = append(buf, chunk...)
	u.match(buf)
	if len(buf) > scanWindow {
		buf = buf[len(buf)-scanWindow:]
	}
	u.tail = append(u.tail[:0], buf...)
}

func (u *usageReader) match(buf []byte) {
	for _, m := range tokenRe.FindAllSubmatch(buf, -1) {
		v, err := strconv.ParseInt(string(m[2]), 10, 64)
		if err != nil {
			continue
		}
		if k := tokenKinds[string(m[1])]; v > u.seen[k] {
			if u.seen == nil {
				u.seen = map[tokenKind]int64{}
			}
			u.seen[k] = v
		}
	}
	if !u.terminal && terminalRe.Match(buf) {
		u.terminal = true
	}
}

// scanFrames reassembles event-stream frames (prelude: total length,
// headers length, CRC; then headers, payload, CRC) and scans their payload,
// and the base64 model chunk it may carry.
func (u *usageReader) scanFrames(chunk []byte) {
	u.buf = append(u.buf, chunk...)
	for len(u.buf) >= 12 {
		total := int(binary.BigEndian.Uint32(u.buf[0:4]))
		headers := int(binary.BigEndian.Uint32(u.buf[4:8]))
		if total < 16 || total > maxFrame || headers > total-16 {
			// Not an event stream after all: fall back to plain scanning.
			u.frames = false
			u.scan(u.buf)
			u.buf = nil
			return
		}
		if len(u.buf) < total {
			return
		}
		payload := u.buf[12+headers : total-4]
		u.match(payload)
		var p struct {
			Bytes string `json:"bytes"`
		}
		if json.Unmarshal(payload, &p) == nil && p.Bytes != "" {
			if dec, err := base64.StdEncoding.DecodeString(p.Bytes); err == nil {
				u.match(dec)
			}
		}
		u.buf = u.buf[total:]
	}
}

// tally is the usage seen. The cache counts of OpenAI-style usage are a
// part of the prompt: they move from the input to the cache counters.
func (u *usageReader) tally() Usage {
	part := u.seen[kindCachedPart] + u.seen[kindWrittenPart]
	return Usage{
		InputTokens:      max(0, u.seen[kindInput]-part),
		OutputTokens:     u.seen[kindOutput],
		CacheReadTokens:  u.seen[kindCacheRead] + u.seen[kindCachedPart],
		CacheWriteTokens: u.seen[kindCacheWrite] + u.seen[kindWrittenPart],
	}
}

// complete reports whether the response reached its end: EOF, or for a
// stream the end it shows (the client may leave right after it).
func (u *usageReader) complete() bool {
	return u.eof || (u.stream && u.terminal)
}

// estimateCut completes the usage of a response cut before its end: the
// input missing from what was seen is read from the size of the request,
// the output is at least a quarter of the bytes received (capped by the
// output the request asked for). Both err high (S-27).
func (u *usageReader) estimateCut(us Usage) Usage {
	estimated := false
	if us.InputTokens+us.CacheReadTokens+us.CacheWriteTokens == 0 && u.reqBytes > 0 {
		us.InputTokens, estimated = u.reqBytes/3+1, true
	}
	out := u.bytes / 4
	if u.maxOutput > 0 {
		out = min(out, u.maxOutput)
	}
	if out > us.OutputTokens {
		us.OutputTokens, estimated = out, true
	}
	if estimated {
		us.Estimated = 1
	}
	return us
}

func (u *usageReader) report() {
	if u.reported {
		return
	}
	u.reported = true
	us := u.tally()
	if u.estimate && !u.complete() {
		us = u.estimateCut(us)
		slog.Warn("credproxy: response cut before its end, usage counted from what was seen",
			"session", u.g.SessionID, "estimated", us.Estimated > 0, "bytes", u.bytes)
	}
	u.g.mu.Lock()
	u.g.usage.InputTokens += us.InputTokens
	u.g.usage.OutputTokens += us.OutputTokens
	u.g.usage.CacheReadTokens += us.CacheReadTokens
	u.g.usage.CacheWriteTokens += us.CacheWriteTokens
	u.g.usage.Estimated += us.Estimated
	u.g.mu.Unlock()
	if u.onReport != nil {
		u.onReport(us)
	}
}
