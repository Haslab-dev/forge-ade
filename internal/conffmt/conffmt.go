// Package conffmt formats configuration file content: pretty JSON, compact
// JSONL, and a comment/order-preserving TOML tidy. Formatters are best-effort
// and never destroy data: invalid JSON errors out, unparseable JSONL lines are
// left untouched, and the TOML tidy only normalizes spacing while keeping
// comments, ordering, and multi-line constructs intact.
package conffmt

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Format formats content based on the path's extension.
func Format(path, content string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return formatJSON(content)
	case ".jsonl":
		return formatJSONL(content), nil
	case ".toml", ".tml":
		return TidyTOML(content), nil
	default:
		return "", fmt.Errorf("no formatter for %s files", filepath.Ext(path))
	}
}

func formatJSON(content string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(content)))
	dec.UseNumber() // keep integer precision in configs
	var sb strings.Builder
	if err := encodeValue(dec, &sb, "", false); err != nil {
		return "", err
	}
	// Reject trailing data after the top-level value.
	if _, err := dec.Token(); err != io.EOF {
		return "", fmt.Errorf("invalid JSON: trailing data")
	}
	sb.WriteString("\n")
	return sb.String(), nil
}

// formatJSONL re-serializes each line compactly (one object per line),
// preserving object key order. Lines that do not parse are passed through
// unchanged so nothing is lost.
func formatJSONL(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(trimmed))
		dec.UseNumber()
		var sb strings.Builder
		if err := encodeValue(dec, &sb, "", true); err != nil {
			continue
		}
		if _, err := dec.Token(); err != io.EOF {
			continue
		}
		lines[i] = sb.String()
	}
	return strings.Join(lines, "\n")
}

// encodeValue re-emits the next JSON value from the token stream with 2-space
// indentation, preserving object key order. compact=true emits single-line.
func encodeValue(dec *json.Decoder, sb *strings.Builder, indent string, compact bool) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return encodeToken(dec, tok, sb, indent, compact)
}

func encodeToken(dec *json.Decoder, tok json.Token, sb *strings.Builder, indent string, compact bool) error {
	if delim, ok := tok.(json.Delim); ok {
		switch delim {
		case '{':
			return encodeContainer(dec, sb, indent, compact, '{', '}')
		case '[':
			return encodeContainer(dec, sb, indent, compact, '[', ']')
		default:
			return fmt.Errorf("invalid JSON: unexpected %q", delim)
		}
	}
	// Scalar: json.Number, string, bool, nil — Marshal handles escaping.
	encoded, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	sb.Write(encoded)
	return nil
}

func encodeContainer(dec *json.Decoder, sb *strings.Builder, indent string, compact bool, open, close json.Delim) error {
	sb.WriteRune(rune(open))
	inner := indent + "  "
	first := true
	for dec.More() {
		if !compact {
			if first {
				sb.WriteString("\n")
			} else {
				sb.WriteString(",\n")
			}
			sb.WriteString(inner)
		} else if !first {
			sb.WriteString(",")
		}
		first = false
		if open == '{' {
			keyTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			key, ok := keyTok.(string)
			if !ok {
				return fmt.Errorf("invalid JSON: object key is not a string")
			}
			encoded, err := json.Marshal(key)
			if err != nil {
				return err
			}
			sb.Write(encoded)
			sb.WriteString(":")
			if !compact {
				sb.WriteString(" ")
			}
		}
		if err := encodeNextValue(dec, sb, inner, compact); err != nil {
			return err
		}
	}
	if !compact && !first {
		sb.WriteString("\n")
		sb.WriteString(indent)
	}
	sb.WriteRune(rune(close))
	_, err := dec.Token() // consume the closing delimiter
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func encodeNextValue(dec *json.Decoder, sb *strings.Builder, indent string, compact bool) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return encodeToken(dec, tok, sb, indent, compact)
}

// TidyTOML normalizes TOML layout without a full parse: key = value spacing,
// trimmed section headers and trailing whitespace, collapsed repeated blank
// lines — while preserving comments, key order, and multi-line constructs
// (multi-line strings and arrays are emitted untouched).
func TidyTOML(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines)+1)
	st := &tomlState{}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")

		if st.multiline {
			out = append(out, line)
			scanTOMLLine(line, st)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// Collapse repeated blank lines; no leading blanks.
			if n := len(out); n > 0 && out[n-1] != "" {
				out = append(out, "")
			}
			continue
		}

		// Inside a multi-line array: emit items as-is (right-trimmed only).
		if st.arrayDepth > 0 {
			out = append(out, line)
			scanTOMLLine(line, st)
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			out = append(out, line)
			continue
		}

		if eq := indexEqOutsideStrings(trimmed); eq > 0 && !isSectionHeader(trimmed) {
			key := strings.TrimRight(trimmed[:eq], " \t")
			val := strings.TrimSpace(trimmed[eq+1:])
			out = append(out, key+" = "+val)
			scanTOMLLine(trimmed, st)
			continue
		}

		// Section headers, bare keys, unknown lines: trimmed as-is.
		out = append(out, trimmed)
		scanTOMLLine(trimmed, st)
	}

	// Trim trailing blank lines, single trailing newline.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	result := strings.Join(out, "\n")
	if result != "" {
		result += "\n"
	}
	return result
}

type tomlState struct {
	multiline  bool
	strDelim   string
	arrayDepth int
}

// isSectionHeader reports whether the trimmed line opens with a table header
// ([a.b] / [[a.b]]) rather than a key assignment.
func isSectionHeader(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "[") {
		return false
	}
	return indexEqOutsideStrings(trimmed) < 0
}

// indexEqOutsideStrings finds the first '=' outside quotes and comments,
// returning -1 when the line has none (or the '=' appears only in a comment).
func indexEqOutsideStrings(line string) int {
	i := 0
	for i < len(line) {
		switch c := line[i]; {
		case c == '"' || c == '\'':
			delim := string(c)
			if strings.HasPrefix(line[i:], delim+delim+delim) {
				return -1 // multi-line string opens on an assignment line
			}
			j := i + 1
			for j < len(line) {
				if line[j] == '\\' && c == '"' {
					j += 2
					continue
				}
				if line[j] == c {
					break
				}
				j++
			}
			i = j + 1
		case c == '#':
			return -1
		case c == '=':
			return i
		default:
			i++
		}
	}
	return -1
}

// scanTOMLLine updates multi-line string / array state.
func scanTOMLLine(line string, st *tomlState) {
	i := 0
	for i < len(line) {
		switch c := line[i]; {
		case st.multiline:
			if strings.HasPrefix(line[i:], st.strDelim) {
				i += len(st.strDelim)
				st.multiline = false
			} else {
				i++
			}
		case c == '"' || c == '\'':
			delim := string(c)
			if strings.HasPrefix(line[i:], delim+delim+delim) {
				rest := line[i+3:]
				if strings.Contains(rest, delim+delim+delim) {
					i += 3 + strings.Index(rest, delim+delim+delim) + 3
				} else {
					st.multiline = true
					st.strDelim = delim + delim + delim
					return
				}
			} else {
				j := i + 1
				for j < len(line) {
					if line[j] == '\\' && c == '"' {
						j += 2
						continue
					}
					if line[j] == c {
						break
					}
					j++
				}
				i = j + 1
			}
		case c == '#':
			return
		case c == '[':
			st.arrayDepth++
			i++
		case c == ']':
			if st.arrayDepth > 0 {
				st.arrayDepth--
			}
			i++
		default:
			i++
		}
	}
}
