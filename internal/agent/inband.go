package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/google/uuid"
)

var (
	// Mimo / Qwen style: <toolcall><function=skill><parameter=skill>impeccable</parameter></function></toolcall>
	reMimoToolCall = regexp.MustCompile(`(?s)<toolcall>\s*<function(?:=|\s+name=)["']?([^"'>\s]+)["']?>(.*?)</function>\s*</toolcall>`)
	reMimoParam    = regexp.MustCompile(`(?s)<parameter(?:=|\s+name=)["']?([^"'>\s]+)["']?>(.*?)</parameter>`)

	// JSON style: <tool_call>{"name": "...", "arguments": {...}}</tool_call>
	reJSONToolCall = regexp.MustCompile(`(?s)<(?:tool_call|toolcall)>\s*(\{.*?\})\s*</(?:tool_call|toolcall)>`)

	// Anthropic invoke / DeepSeek DSML style:
	// <invoke name="...">...<parameter name="...">...</parameter></invoke>
	// or < | | DSML | | invoke name="...">...< | | DSML | | parameter name="...">...</ | | DSML | | parameter></ | | DSML | | invoke>
	reInvokeToolCall = regexp.MustCompile(`(?si)<[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?invoke\s+name=["']?([^"'>\s]+)["']?[^>]*>(.*?)</?[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?invoke\s*>`)
	reInvokeParam    = regexp.MustCompile(`(?si)<[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?parameter(?:\s+name=|\s*=\s*)["']?([^"'>\s]+)["']?[^>]*>(.*?)</?[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?parameter\s*>`)

	// DeepSeek DSML & in-band container tags: < | | DSML | | calls> ... </ | | DSML | | calls>
	reDSMLContainer = regexp.MustCompile(`(?si)</?[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*(?:calls|call)?|call_begin|call_end)[|\s\x{FF5C}]*>`)

	// Function call style: <function_call name="..."><parameter=...></function_call>
	reFuncCallNamed = regexp.MustCompile(`(?s)<function_call\s+name=["']?([^"'>\s]+)["']?>(.*?)</function_call>`)
	reFuncCallXML   = regexp.MustCompile(`(?s)<function_call>\s*<name>([^<]+)</name>\s*(?:<arguments>(\{.*?\})</arguments>|(.*?))\s*</function_call>`)

	// Generic cleanup for residual open/unclosed tool tags
	reResidualToolTags = regexp.MustCompile(`(?si)</?[|\s\x{FF5C}]*(?:toolcall|tool_call|function_call|invoke|parameter|DSML[|\s\x{FF5C}]*(?:calls|call)?|call_begin|call_end)(?:=[^>]+|[^>]*)>`)
)

// parseInbandToolCalls extracts in-band tool call syntax from models that emit
// XML or JSON tool calls directly in text content (e.g. Mimo, DeepSeek, Qwen,
// Anthropic in-band). It returns the parsed tool calls and the text with those
// tags cleanly stripped so raw tool syntax never leaks to the user.
func parseInbandToolCalls(text string) ([]*llm.ToolCall, string) {
	if !strings.Contains(text, "<") {
		return nil, text
	}

	var calls []*llm.ToolCall
	cleaned := text

	// 1. Mimo / Qwen format: <toolcall><function=NAME><parameter=KEY>VAL</parameter></function></toolcall>
	for {
		loc := reMimoToolCall.FindStringSubmatchIndex(cleaned)
		if loc == nil {
			break
		}
		matchStr := cleaned[loc[0]:loc[1]]
		m := reMimoToolCall.FindStringSubmatch(matchStr)
		if len(m) >= 3 {
			fnName := strings.TrimSpace(m[1])
			body := m[2]
			args := make(map[string]any)
			for _, pm := range reMimoParam.FindAllStringSubmatch(body, -1) {
				if len(pm) >= 3 {
					pKey := strings.TrimSpace(pm[1])
					pVal := strings.TrimSpace(pm[2])
					var jsonVal any
					if (strings.HasPrefix(pVal, "{") || strings.HasPrefix(pVal, "[")) && json.Unmarshal([]byte(pVal), &jsonVal) == nil {
						args[pKey] = jsonVal
					} else {
						args[pKey] = pVal
					}
				}
			}
			if fnName == "skill" || fnName == "load_skill" {
				if v, ok := args["skill"]; ok && args["name"] == nil {
					args["name"] = v
				}
				if v, ok := args["name"]; ok && args["skill"] == nil {
					args["skill"] = v
				}
			}
			argsJSON, _ := json.Marshal(args)
			calls = append(calls, &llm.ToolCall{
				ID:   fmt.Sprintf("call_inband_%s", uuid.New().String()[:8]),
				Type: "function",
				Function: llm.ToolFunction{
					Name:      fnName,
					Arguments: string(argsJSON),
				},
			})
		}
		cleaned = cleaned[:loc[0]] + cleaned[loc[1]:]
	}

	// 2. JSON format: <tool_call>{"name": "...", "arguments": {...}}</tool_call>
	for {
		loc := reJSONToolCall.FindStringSubmatchIndex(cleaned)
		if loc == nil {
			break
		}
		matchStr := cleaned[loc[0]:loc[1]]
		m := reJSONToolCall.FindStringSubmatch(matchStr)
		if len(m) >= 2 {
			var parsed struct {
				Name      string `json:"name"`
				Arguments any    `json:"arguments"`
			}
			if json.Unmarshal([]byte(m[1]), &parsed) == nil && parsed.Name != "" {
				var argsStr string
				if s, ok := parsed.Arguments.(string); ok {
					argsStr = s
				} else if parsed.Arguments != nil {
					b, _ := json.Marshal(parsed.Arguments)
					argsStr = string(b)
				}
				calls = append(calls, &llm.ToolCall{
					ID:   fmt.Sprintf("call_inband_%s", uuid.New().String()[:8]),
					Type: "function",
					Function: llm.ToolFunction{
						Name:      parsed.Name,
						Arguments: argsStr,
					},
				})
			}
		}
		cleaned = cleaned[:loc[0]] + cleaned[loc[1]:]
	}

	// 3. Anthropic invoke format: <invoke name="..."><parameter name="...">...</parameter></invoke>
	for {
		loc := reInvokeToolCall.FindStringSubmatchIndex(cleaned)
		if loc == nil {
			break
		}
		matchStr := cleaned[loc[0]:loc[1]]
		m := reInvokeToolCall.FindStringSubmatch(matchStr)
		if len(m) >= 3 {
			fnName := strings.TrimSpace(m[1])
			body := m[2]
			args := make(map[string]any)
			for _, pm := range reInvokeParam.FindAllStringSubmatch(body, -1) {
				if len(pm) >= 3 {
					pKey := strings.TrimSpace(pm[1])
					pVal := strings.TrimSpace(pm[2])
					var jsonVal any
					if (strings.HasPrefix(pVal, "{") || strings.HasPrefix(pVal, "[")) && json.Unmarshal([]byte(pVal), &jsonVal) == nil {
						args[pKey] = jsonVal
					} else {
						args[pKey] = pVal
					}
				}
			}
			if fnName == "skill" || fnName == "load_skill" {
				if v, ok := args["skill"]; ok && args["name"] == nil {
					args["name"] = v
				}
				if v, ok := args["name"]; ok && args["skill"] == nil {
					args["skill"] = v
				}
			}
			argsJSON, _ := json.Marshal(args)
			calls = append(calls, &llm.ToolCall{
				ID:   fmt.Sprintf("call_inband_%s", uuid.New().String()[:8]),
				Type: "function",
				Function: llm.ToolFunction{
					Name:      fnName,
					Arguments: string(argsJSON),
				},
			})
		}
		cleaned = cleaned[:loc[0]] + cleaned[loc[1]:]
	}

	// 4. Function call format: <function_call name="...">...</function_call>
	for {
		loc := reFuncCallNamed.FindStringSubmatchIndex(cleaned)
		if loc == nil {
			break
		}
		matchStr := cleaned[loc[0]:loc[1]]
		m := reFuncCallNamed.FindStringSubmatch(matchStr)
		if len(m) >= 3 {
			fnName := strings.TrimSpace(m[1])
			body := m[2]
			args := make(map[string]any)
			for _, pm := range reMimoParam.FindAllStringSubmatch(body, -1) {
				if len(pm) >= 3 {
					args[strings.TrimSpace(pm[1])] = strings.TrimSpace(pm[2])
				}
			}
			argsJSON, _ := json.Marshal(args)
			calls = append(calls, &llm.ToolCall{
				ID:   fmt.Sprintf("call_inband_%s", uuid.New().String()[:8]),
				Type: "function",
				Function: llm.ToolFunction{
					Name:      fnName,
					Arguments: string(argsJSON),
				},
			})
		}
		cleaned = cleaned[:loc[0]] + cleaned[loc[1]:]
	}

	// 5. XML function call: <function_call><name>...</name><arguments>...</arguments></function_call>
	for {
		loc := reFuncCallXML.FindStringSubmatchIndex(cleaned)
		if loc == nil {
			break
		}
		matchStr := cleaned[loc[0]:loc[1]]
		m := reFuncCallXML.FindStringSubmatch(matchStr)
		if len(m) >= 2 {
			fnName := strings.TrimSpace(m[1])
			argsStr := ""
			if len(m) >= 3 {
				argsStr = strings.TrimSpace(m[2])
			}
			if argsStr == "" && len(m) >= 4 {
				argsStr = strings.TrimSpace(m[3])
			}
			calls = append(calls, &llm.ToolCall{
				ID:   fmt.Sprintf("call_inband_%s", uuid.New().String()[:8]),
				Type: "function",
				Function: llm.ToolFunction{
					Name:      fnName,
					Arguments: argsStr,
				},
			})
		}
		cleaned = cleaned[:loc[0]] + cleaned[loc[1]:]
	}

	if len(calls) > 0 {
		cleaned = reDSMLContainer.ReplaceAllString(cleaned, "")
		cleaned = reResidualToolTags.ReplaceAllString(cleaned, "")
		return calls, strings.TrimSpace(cleaned)
	}

	return nil, text
}

// cleanInbandToolSyntax strips residual in-band tool call tags, DSML containers, and unclosed markup.
func cleanInbandToolSyntax(text string) string {
	if !strings.Contains(text, "<") {
		return text
	}
	cleaned := text
	cleaned = reInvokeToolCall.ReplaceAllString(cleaned, "")
	cleaned = reMimoToolCall.ReplaceAllString(cleaned, "")
	cleaned = reJSONToolCall.ReplaceAllString(cleaned, "")
	cleaned = reFuncCallNamed.ReplaceAllString(cleaned, "")
	cleaned = reFuncCallXML.ReplaceAllString(cleaned, "")
	cleaned = reDSMLContainer.ReplaceAllString(cleaned, "")
	cleaned = reResidualToolTags.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}
