package agent

import "strings"

type thoughtEvent struct {
	kind string // "text" or "thinking"
	text string
}

// thoughtStreamScanner extracts in-band thought tags (<think>, <thought>, <thinking>, <scratchpad>)
// from a streaming text LLM response, separating internal reasoning from user-visible text.
type thoughtStreamScanner struct {
	inThinking bool
	buf        strings.Builder
	events     []thoughtEvent
}

func newThoughtStreamScanner() *thoughtStreamScanner {
	return &thoughtStreamScanner{}
}

func isThoughtTag(name string) bool {
	name = strings.TrimSpace(strings.ToLower(name))
	return name == "think" || name == "thought" || name == "thinking" || name == "scratchpad"
}

// feed processes a streamed delta text chunk and returns the parsed events.
func (s *thoughtStreamScanner) feed(chunk string) []thoughtEvent {
	if chunk == "" {
		return nil
	}
	s.buf.WriteString(chunk)
	s.events = s.events[:0]
	s.consume()

	out := make([]thoughtEvent, len(s.events))
	copy(out, s.events)
	return out
}

// flush finalizes any remaining buffered content at the end of the stream.
func (s *thoughtStreamScanner) flush() []thoughtEvent {
	s.events = s.events[:0]
	rem := s.buf.String()
	s.buf.Reset()
	if rem != "" {
		if s.inThinking {
			s.events = append(s.events, thoughtEvent{kind: "thinking", text: rem})
		} else {
			s.events = append(s.events, thoughtEvent{kind: "text", text: rem})
		}
	}
	s.inThinking = false

	out := make([]thoughtEvent, len(s.events))
	copy(out, s.events)
	return out
}

func (s *thoughtStreamScanner) consume() {
	for s.buf.Len() > 0 {
		buf := s.buf.String()

		if !s.inThinking {
			idx := strings.Index(buf, "<")
			if idx < 0 {
				s.events = append(s.events, thoughtEvent{kind: "text", text: buf})
				s.buf.Reset()
				return
			}
			if idx > 0 {
				s.events = append(s.events, thoughtEvent{kind: "text", text: buf[:idx]})
				s.buf.Reset()
				s.buf.WriteString(buf[idx:])
				buf = s.buf.String()
			}

			// Starts with '<'
			gtIdx := strings.Index(buf, ">")
			if gtIdx < 0 {
				// Incomplete tag. If it's too long to be a thought tag, flush as text.
				if len(buf) > 30 {
					s.events = append(s.events, thoughtEvent{kind: "text", text: buf[:1]})
					s.buf.Reset()
					s.buf.WriteString(buf[1:])
					continue
				}
				// Wait for next chunk to complete tag
				return
			}

			tag := buf[:gtIdx+1]
			inner := strings.TrimSpace(tag[1 : len(tag)-1])
			// Check if it's an opening tag (not closing)
			if !strings.HasPrefix(inner, "/") {
				// Handle potential attributes like <thinking id="1">
				parts := strings.Fields(inner)
				if len(parts) > 0 && isThoughtTag(parts[0]) {
					s.inThinking = true
					s.buf.Reset()
					s.buf.WriteString(buf[gtIdx+1:])
					continue
				}
			}

			// Normal HTML/XML tag or TS generic (e.g. <div> or <T>)
			s.events = append(s.events, thoughtEvent{kind: "text", text: tag})
			s.buf.Reset()
			s.buf.WriteString(buf[gtIdx+1:])
		} else {
			// In thinking mode: look for closing tag </...
			idx := strings.Index(buf, "</")
			if idx < 0 {
				// Might end with incomplete '<' or '</'
				if strings.HasSuffix(buf, "<") {
					if len(buf) > 1 {
						s.events = append(s.events, thoughtEvent{kind: "thinking", text: buf[:len(buf)-1]})
						s.buf.Reset()
						s.buf.WriteString("<")
					}
					return
				}
				s.events = append(s.events, thoughtEvent{kind: "thinking", text: buf})
				s.buf.Reset()
				return
			}

			if idx > 0 {
				s.events = append(s.events, thoughtEvent{kind: "thinking", text: buf[:idx]})
				s.buf.Reset()
				s.buf.WriteString(buf[idx:])
				buf = s.buf.String()
			}

			// Starts with '</'
			gtIdx := strings.Index(buf, ">")
			if gtIdx < 0 {
				if len(buf) > 30 {
					s.events = append(s.events, thoughtEvent{kind: "thinking", text: buf[:2]})
					s.buf.Reset()
					s.buf.WriteString(buf[2:])
					continue
				}
				// Wait for next chunk
				return
			}

			tag := buf[:gtIdx+1]
			inner := strings.TrimSpace(tag[2 : len(tag)-1])
			parts := strings.Fields(inner)
			if len(parts) > 0 && isThoughtTag(parts[0]) {
				s.inThinking = false
				s.buf.Reset()
				s.buf.WriteString(buf[gtIdx+1:])
				continue
			}

			// Not a thought closing tag
			s.events = append(s.events, thoughtEvent{kind: "thinking", text: tag})
			s.buf.Reset()
			s.buf.WriteString(buf[gtIdx+1:])
		}
	}
}
