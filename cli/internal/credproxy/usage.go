package credproxy

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
)

// Usage fields across providers:
//   - Bedrock Converse (event-stream frames carry JSON): "inputTokens", "outputTokens"
//   - Bedrock InvokeModel with response stream: event-stream frames whose JSON
//     payload carries the model chunk in base64 ({"bytes": "…"}); Anthropic
//     chunks inside: "input_tokens", "output_tokens", and the final
//     "amazon-bedrock-invocationMetrics" "inputTokenCount", "outputTokenCount"
//   - Anthropic Messages (SSE): "input_tokens", "output_tokens" (message_delta is cumulative)
//   - OpenAI / OpenRouter: "prompt_tokens", "completion_tokens" (streams carry
//     them only with stream_options.include_usage, added by the proxy)
var (
	inputRe  = regexp.MustCompile(`"(?:inputTokens|input_tokens|prompt_tokens|inputTokenCount)"\s*:\s*(\d+)`)
	outputRe = regexp.MustCompile(`"(?:outputTokens|output_tokens|completion_tokens|outputTokenCount)"\s*:\s*(\d+)`)
)

const scanWindow = 4096

// maxFrame bounds an AWS event-stream frame (the service limit is 16 MiB).
const maxFrame = 16 << 20

// usageReader passes the response through unchanged while scanning it for
// usage counters. Per response, the highest value seen for each counter is
// accounted (streams repeat or accumulate counters).
type usageReader struct {
	rc       io.ReadCloser
	g        *grantState
	onReport func(in, out int64) // accounts the response (Proxy.account)
	tail     []byte
	maxIn    int64
	maxOut   int64
	reported bool
	// frames decodes AWS event-stream frames with base64 payloads
	// (Bedrock invoke-with-response-stream).
	frames bool
	buf    []byte
}

func (u *usageReader) Read(p []byte) (int, error) {
	n, err := u.rc.Read(p)
	if n > 0 {
		if u.frames {
			u.scanFrames(p[:n])
		} else {
			u.scan(p[:n])
		}
	}
	if err == io.EOF {
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
	for _, m := range inputRe.FindAllSubmatch(buf, -1) {
		if v, err := strconv.ParseInt(string(m[1]), 10, 64); err == nil && v > u.maxIn {
			u.maxIn = v
		}
	}
	for _, m := range outputRe.FindAllSubmatch(buf, -1) {
		if v, err := strconv.ParseInt(string(m[1]), 10, 64); err == nil && v > u.maxOut {
			u.maxOut = v
		}
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

func (u *usageReader) report() {
	if u.reported {
		return
	}
	u.reported = true
	u.g.mu.Lock()
	u.g.usage.InputTokens += u.maxIn
	u.g.usage.OutputTokens += u.maxOut
	u.g.mu.Unlock()
	if u.onReport != nil && (u.maxIn > 0 || u.maxOut > 0) {
		u.onReport(u.maxIn, u.maxOut)
	}
}
