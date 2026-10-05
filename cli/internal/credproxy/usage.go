package credproxy

import (
	"io"
	"regexp"
	"strconv"
)

// Usage fields across providers:
//   - Bedrock Converse (event-stream frames carry JSON): "inputTokens", "outputTokens"
//   - Anthropic Messages (SSE): "input_tokens", "output_tokens" (message_delta is cumulative)
//   - OpenAI / OpenRouter: "prompt_tokens", "completion_tokens"
var (
	inputRe  = regexp.MustCompile(`"(?:inputTokens|input_tokens|prompt_tokens)"\s*:\s*(\d+)`)
	outputRe = regexp.MustCompile(`"(?:outputTokens|output_tokens|completion_tokens)"\s*:\s*(\d+)`)
)

const scanWindow = 4096

// usageReader passes the response through unchanged while scanning it for
// usage counters. Per response, the highest value seen for each counter is
// accounted (streams repeat or accumulate counters).
type usageReader struct {
	rc       io.ReadCloser
	g        *grantState
	tail     []byte
	maxIn    int64
	maxOut   int64
	reported bool
}

func (u *usageReader) Read(p []byte) (int, error) {
	n, err := u.rc.Read(p)
	if n > 0 {
		u.scan(p[:n])
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
	if len(buf) > scanWindow {
		buf = buf[len(buf)-scanWindow:]
	}
	u.tail = append(u.tail[:0], buf...)
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
}
