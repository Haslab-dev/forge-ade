package agent

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/llm"
)

// In-band (owned) tool-calling dialect.
// packages/ai/src/dialect/. When a session uses a dialect, the agent sends NO
// native `tools` array; instead the tool catalog is described in the system
// prompt, prior tool calls/results are re-encoded as XML text, and the model's
// text output is parsed back into tool calls by a streaming scanner.

// dialectPrompt is injected into the system prompt when XML dialect mode is
// active. It teaches the model the exact in-band grammar.
const xmlDialectPrompt = `
## Tool calling format
You call tools by emitting XML blocks in your reply, never a JSON tools array:

<invoke name="tool_name">
<parameter name="arg1">value</parameter>
<parameter name="arg2">{"json": "object"}</parameter>
</invoke>

- Each tool invocation must be a complete <invoke>...</invoke> block.
- Every argument is a <parameter name="...">...</parameter> element. String values
  are written as plain text; objects/arrays/numbers/booleans are written as JSON.
- You may emit multiple invokes; they run sequentially in order.
- When you need to think, wrap internal reasoning in <thinking>...</thinking>.
- After a tool runs you receive its result as <tool_response>...</tool_response>
  in a Human message. Use the result and continue.

Available tools:
`

// xmlDialectScanner is a streaming scanner that parses the XML in-band tool
// grammar from incremental text chunks.
// AnthropicInbandScanner (subset: invoke/parameter/thinking + wrapper tags).
type xmlDialectScanner struct {
	buf       strings.Builder
	state     int // 0 outside, 1 invoke, 2 parameter, 3 thinking
	returnSt  int
	toolID    string
	toolName  string
	args      map[string]any
	paramName string
	paramVal  strings.Builder
	events    []dialectEvent
}

type dialectEvent struct {
	kind     string // "text", "tool_start", "tool_arg", "tool_end", "thinking"
	text     string
	toolID   string
	toolName string
	argName  string
	args     map[string]any
}

const (
	dialectStateOutside = iota
	dialectStateInvoke
	dialectStateParameter
	dialectStateThinking
)

func newXMLDialectScanner() *xmlDialectScanner {
	return &xmlDialectScanner{
		state: dialectStateOutside,
		args:  make(map[string]any),
	}
}

// feed processes a text chunk and returns any parse events it produced.
func (s *xmlDialectScanner) feed(text string) []dialectEvent {
	s.buf.WriteString(text)
	s.events = s.events[:0]
	s.consume(false)
	out := make([]dialectEvent, len(s.events))
	copy(out, s.events)
	return out
}

// flush processes any trailing buffered input at end of stream.
func (s *xmlDialectScanner) flush() []dialectEvent {
	s.events = s.events[:0]
	s.consume(true)
	out := make([]dialectEvent, len(s.events))
	copy(out, s.events)
	s.buf.Reset()
	return out
}

func (s *xmlDialectScanner) consume(final bool) {
	for s.buf.Len() > 0 {
		progressed := true
		switch s.state {
		case dialectStateOutside:
			progressed = s.consumeOutside(final)
		case dialectStateInvoke:
			progressed = s.consumeInvoke(final)
		case dialectStateParameter:
			progressed = s.consumeParameter(final)
		case dialectStateThinking:
			progressed = s.consumeThinking(final)
		}
		if !progressed {
			break
		}
	}
	if final {
		s.finishFinal()
	}
}

func (s *xmlDialectScanner) consumeOutside(final bool) bool {
	buf := s.buf.String()
	idx := strings.Index(buf, "<")
	if idx < 0 {
		if buf != "" {
			s.emitText(buf)
		}
		s.buf.Reset()
		return false
	}
	if idx > 0 {
		s.emitText(buf[:idx])
		s.buf.Reset()
		s.buf.WriteString(buf[idx:])
		return true
	}
	// Buffer starts with '<' — try to read a tag.
	tag, ok := s.readTag(final)
	if !ok {
		return false
	}
	if tag == "" {
		// Not a tag we recognize: emit the '<' as literal text.
		s.emitText("<")
		s.buf.Reset()
		s.buf.WriteString(buf[1:])
		return true
	}
	// Recognized tag.
	open, closing, name, attrs := parseTag(tag)
	switch {
	case open && name == "invoke":
		s.buf.Reset()
		s.buf.WriteString(buf[len(tag):])
		s.startInvoke(attrs)
	case open && (name == "thinking" || name == "think" || name == "scratchpad"):
		s.buf.Reset()
		s.buf.WriteString(buf[len(tag):])
		s.state = dialectStateThinking
		s.returnSt = dialectStateOutside
	default:
		// wrapper open/close (<function_calls> etc.) — skip.
		s.buf.Reset()
		s.buf.WriteString(buf[len(tag):])
		_ = closing
	}
	return true
}

func (s *xmlDialectScanner) consumeInvoke(final bool) bool {
	buf := s.buf.String()
	idx := strings.Index(buf, "<")
	if idx < 0 {
		if final {
			s.resetCall()
			return false
		}
		return false // wait for more input
	}
	if idx > 0 {
		s.buf.Reset()
		s.buf.WriteString(buf[idx:])
		return true
	}
	tag, ok := s.readTag(final)
	if !ok {
		return false
	}
	if tag == "" {
		s.buf.Reset()
		s.buf.WriteString(buf[1:])
		return true
	}
	open, closing, name, attrs := parseTag(tag)
	s.buf.Reset()
	s.buf.WriteString(buf[len(tag):])
	switch {
	case closing && name == "invoke":
		s.endInvoke()
	case open && name == "parameter":
		s.startParameter(attrs)
	}
	return true
}

func (s *xmlDialectScanner) consumeParameter(final bool) bool {
	buf := s.buf.String()
	idx := strings.Index(buf, "<")
	if idx < 0 {
		s.paramVal.WriteString(buf)
		s.buf.Reset()
		return false
	}
	if idx > 0 {
		s.paramVal.WriteString(buf[:idx])
		s.buf.Reset()
		s.buf.WriteString(buf[idx:])
		return true
	}
	tag, ok := s.readTag(final)
	if !ok {
		return false
	}
	if tag == "" {
		s.paramVal.WriteString("<")
		s.buf.Reset()
		s.buf.WriteString(buf[1:])
		return true
	}
	open, closing, name, _ := parseTag(tag)
	s.buf.Reset()
	s.buf.WriteString(buf[len(tag):])
	if closing && name == "parameter" {
		s.finishParameter()
	} else if !open {
		s.paramVal.WriteString(tag)
	}
	return true
}

func (s *xmlDialectScanner) consumeThinking(final bool) bool {
	buf := s.buf.String()
	idx := strings.Index(buf, "<")
	if idx < 0 {
		if buf != "" {
			s.emitThinking(buf)
		}
		s.buf.Reset()
		return false
	}
	if idx > 0 {
		s.emitThinking(buf[:idx])
		s.buf.Reset()
		s.buf.WriteString(buf[idx:])
		return true
	}
	tag, ok := s.readTag(final)
	if !ok {
		return false
	}
	if tag == "" {
		s.emitThinking("<")
		s.buf.Reset()
		s.buf.WriteString(buf[1:])
		return true
	}
	_, closing, name, _ := parseTag(tag)
	s.buf.Reset()
	s.buf.WriteString(buf[len(tag):])
	if closing && (name == "thinking" || name == "think" || name == "scratchpad") {
		s.state = s.returnSt
	}
	return true
}

func (s *xmlDialectScanner) finishFinal() {
	switch s.state {
	case dialectStateThinking:
		s.state = s.returnSt
	case dialectStateInvoke, dialectStateParameter:
		s.resetCall()
	}
	s.state = dialectStateOutside
	s.buf.Reset()
}

// readTag attempts to parse a full tag starting at the buffer's '<'. Returns
// ("", false) when the tag is incomplete, ("", true) when it is not a tag at
// all, and the raw tag string otherwise.
func (s *xmlDialectScanner) readTag(final bool) (string, bool) {
	buf := s.buf.String()
	if len(buf) < 2 {
		if final {
			return "", true
		}
		return "", false
	}
	end := strings.IndexByte(buf, '>')
	if end < 0 {
		if final {
			return "", true
		}
		return "", false
	}
	raw := buf[:end+1]
	trimmed := strings.TrimSpace(raw[1 : len(raw)-1])
	if trimmed == "" {
		return "", true
	}
	// A tag must start with a letter, '/', or '!'.
	c := trimmed[0]
	if c != '/' && c != '!' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') {
		return "", true
	}
	return raw, true
}

// parseTag decomposes a raw XML tag into open/closing/name/attributes.
func parseTag(raw string) (open bool, closing bool, name string, attrs map[string]string) {
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	closing = strings.HasPrefix(inner, "/")
	if closing {
		inner = strings.TrimSpace(strings.TrimPrefix(inner, "/"))
	}
	name = inner
	attrs = map[string]string{}
	if sp := strings.IndexAny(inner, " \t\r\n"); sp >= 0 {
		name = inner[:sp]
		rest := inner[sp+1:]
		for _, part := range splitAttrs(rest) {
			eq := strings.IndexByte(part, '=')
			if eq < 0 {
				continue
			}
			key := strings.TrimSpace(part[:eq])
			val := strings.TrimSpace(part[eq+1:])
			val = strings.Trim(val, `"'`)
			attrs[key] = val
		}
	}
	name = strings.ToLower(name)
	return true, closing, name, attrs
}

func splitAttrs(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			cur.WriteByte(c)
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			inQuote = c
			cur.WriteByte(c)
		case ' ', '\t', '\r', '\n':
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func (s *xmlDialectScanner) startInvoke(attrs map[string]string) {
	s.toolID = uuid.New().String()
	s.toolName = strings.TrimSpace(attrs["name"])
	s.args = make(map[string]any)
	s.state = dialectStateInvoke
	s.returnSt = dialectStateOutside
	if s.toolName != "" {
		s.events = append(s.events, dialectEvent{
			kind:     "tool_start",
			toolID:   s.toolID,
			toolName: s.toolName,
		})
	}
}

func (s *xmlDialectScanner) startParameter(attrs map[string]string) {
	s.paramName = strings.TrimSpace(attrs["name"])
	s.paramVal.Reset()
	s.state = dialectStateParameter
	s.returnSt = dialectStateInvoke
}

func (s *xmlDialectScanner) finishParameter() {
	val := s.paramVal.String()
	s.paramVal.Reset()
	if s.paramName != "" {
		s.args[s.paramName] = parseParamValue(val)
	}
	s.state = dialectStateInvoke
}

func (s *xmlDialectScanner) endInvoke() {
	if s.toolName != "" {
		s.events = append(s.events, dialectEvent{
			kind:     "tool_end",
			toolID:   s.toolID,
			toolName: s.toolName,
			args:     s.args,
		})
	}
	s.resetCall()
}

func (s *xmlDialectScanner) resetCall() {
	s.state = dialectStateOutside
	s.toolID = ""
	s.toolName = ""
	s.args = make(map[string]any)
	s.paramName = ""
	s.paramVal.Reset()
}

func (s *xmlDialectScanner) emitText(text string) {
	if text == "" {
		return
	}
	s.events = append(s.events, dialectEvent{kind: "text", text: text})
}

func (s *xmlDialectScanner) emitThinking(text string) {
	if text == "" {
		return
	}
	s.events = append(s.events, dialectEvent{kind: "thinking", text: text})
}

// parseParamValue interprets a parameter body: "true"/"false"/numbers parse as
// JSON scalars, otherwise we try JSON and fall back to the raw string.
func parseParamValue(v string) any {
	switch strings.TrimSpace(v) {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	var out any
	if jsonUnmarshalAny([]byte(v), &out) == nil {
		// A bare string like `hello` is not valid JSON, so only accept
		// objects/arrays/numbers from the JSON path.
		switch out.(type) {
		case map[string]any, []any, float64, bool:
			return out
		}
	}
	return v
}

// ---------------------------------------------------------------------------
// Rendering (transcript re-encode)
// ---------------------------------------------------------------------------

func escapeXMLText(v string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(v)
}

func escapeXMLAttr(v string) string {
	r := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	return r.Replace(v)
}

func renderInvokeBlock(name string, args map[string]any) string {
	var sb strings.Builder
	sb.WriteString("<invoke name=\"")
	sb.WriteString(escapeXMLAttr(name))
	sb.WriteString("\">")
	for k, v := range args {
		sb.WriteString("<parameter name=\"")
		sb.WriteString(escapeXMLAttr(k))
		sb.WriteString("\">")
		if s, ok := v.(string); ok {
			sb.WriteString(escapeXMLText(s))
		} else {
			if b, err := jsonMarshalAny(v); err == nil {
				sb.WriteString(string(b))
			} else {
				sb.WriteString(escapeXMLText(fmt.Sprintf("%v", v)))
			}
		}
		sb.WriteString("</parameter>")
	}
	sb.WriteString("</invoke>")
	return sb.String()
}

func renderToolResponse(text string) string {
	return "<tool_response>\n" + text + "\n</tool_response>"
}

// renderDialectTranscript converts session messages into the in-band text
// transcript sent to the model (port of renderLegacyTextTranscript).
func renderDialectTranscript(msgs []AgentMessage, toolCatalog string) string {
	var sb strings.Builder
	sb.WriteString(xmlDialectPrompt)
	sb.WriteString(toolCatalog)
	sb.WriteString("\n\n## Conversation\n")
	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			t := msg.Text()
			if t != "" {
				sb.WriteString("Human: ")
				sb.WriteString(t)
				sb.WriteString("\n\n")
			}
		case "assistant":
			thinking := msg.Thinking()
			text := msg.Text()
			var sb2 strings.Builder
			if thinking != "" {
				sb2.WriteString("<thinking>\n")
				sb2.WriteString(thinking)
				sb2.WriteString("\n</thinking>\n")
			}
			sb2.WriteString(text)
			for _, tc := range msg.ToolCallBlocks() {
				sb2.WriteString(renderInvokeBlock(tc.Name, tc.Arguments))
				sb2.WriteString("\n")
			}
			body := sb2.String()
			if strings.TrimSpace(body) != "" {
				sb.WriteString("Assistant: ")
				sb.WriteString(strings.TrimSpace(body))
				sb.WriteString("\n\n")
			}
		case "tool":
			for _, tr := range msg.ToolResultBlocks() {
				sb.WriteString("Human: ")
				sb.WriteString(renderToolResponse(tr.Text))
				sb.WriteString("\n\n")
			}
		}
	}
	return sb.String()
}

// toolCatalog renders the tool list as text for the dialect prompt.
func toolCatalogText(defs []llm.ToolDefinition) string {
	var sb strings.Builder
	for _, d := range defs {
		sb.WriteString("- ")
		sb.WriteString(d.Function.Name)
		sb.WriteString(": ")
		sb.WriteString(d.Function.Description)
		sb.WriteString("\n")
		if b, err := jsonMarshalAny(d.Function.Parameters); err == nil {
			sb.WriteString("  parameters: ")
			sb.WriteString(string(b))
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// dialectToolIndex finds the toolCallBuf index whose ID matches the given
// tool id, returning (index, true) or (0, false).
func dialectToolIndex(buf map[int]*llm.ToolCall, id string) (int, bool) {
	for idx, tc := range buf {
		if tc.ID == id {
			return idx, true
		}
	}
	return 0, false
}
