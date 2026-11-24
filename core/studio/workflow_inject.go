package studio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// WorkflowVariable represents an injectable parameter inside a reusable
// ForgeADE workflow JSON template (e.g. {{prompt}}, {{style}}, {{model}}, {{seed}}).
type WorkflowVariable struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	ValueType string `json:"valueType"` // "string", "multiline", "int", "float", "file"
	Value     string `json:"value"`
}

var workflowVarRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// ParseWorkflowVariables scans a workflow JSON template for {{variable}} tokens
// (and any explicit "variables" defaults object) and returns editable fields.
func ParseWorkflowVariables(rawJSON string) ([]WorkflowVariable, error) {
	trimmed := strings.TrimSpace(rawJSON)
	if trimmed == "" {
		return nil, fmt.Errorf("empty workflow JSON")
	}

	// Validate JSON structure (replacing {{var}} with placeholder strings if needed).
	probe := workflowVarRe.ReplaceAllString(trimmed, "placeholder")
	var probeObj map[string]interface{}
	if err := json.Unmarshal([]byte(probe), &probeObj); err != nil {
		return nil, fmt.Errorf("invalid workflow JSON: %w", err)
	}

	// Read optional default values from top-level "variables" object.
	defaults := map[string]string{}
	if rawVars, ok := probeObj["variables"].(map[string]interface{}); ok {
		for k, v := range rawVars {
			defaults[k] = fmt.Sprintf("%v", v)
		}
	}

	seen := map[string]bool{}
	var vars []WorkflowVariable

	addVar := func(key, defaultVal string) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		val := defaultVal
		if d, ok := defaults[key]; ok && d != "" && d != "placeholder" {
			val = d
		}
		vars = append(vars, WorkflowVariable{
			Key:       key,
			Label:     humanizeWorkflowKey(key),
			ValueType: inferVariableType(key),
			Value:     val,
		})
	}

	// 1. Scan all {{variable}} placeholders in the JSON.
	for _, m := range workflowVarRe.FindAllStringSubmatch(trimmed, -1) {
		if len(m) >= 2 {
			addVar(m[1], "")
		}
	}

	// 2. Also include any keys declared in top-level "variables" map.
	var defKeys []string
	for k := range defaults {
		defKeys = append(defKeys, k)
	}
	sort.Strings(defKeys)
	for _, k := range defKeys {
		addVar(k, defaults[k])
	}

	sort.SliceStable(vars, func(i, j int) bool {
		return variableSortRank(vars[i]) < variableSortRank(vars[j])
	})

	return vars, nil
}

// InjectWorkflowJSON replaces all {{variable}} placeholders in rawJSON with the
// current values from vars and returns the resolved WorkflowPipeline + JSON bytes.
func InjectWorkflowJSON(rawJSON string, vars []WorkflowVariable) ([]byte, *WorkflowPipeline, error) {
	working := strings.TrimSpace(rawJSON)
	if working == "" {
		return nil, nil, fmt.Errorf("empty workflow JSON")
	}

	for _, v := range vars {
		if v.Key == "" {
			continue
		}
		escaped := strings.ReplaceAll(v.Value, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		escaped = strings.ReplaceAll(escaped, "\n", `\n`)
		re := regexp.MustCompile(`\{\{\s*` + regexp.QuoteMeta(v.Key) + `\s*\}\}`)
		working = re.ReplaceAllString(working, escaped)
	}

	// Strip top-level "variables" if it is a key-value defaults map so unmarshaling
	// into WorkflowPipeline (whose Variables field is []WorkflowVariable) succeeds.
	var rawObj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(working), &rawObj); err != nil {
		return nil, nil, fmt.Errorf("resolved workflow JSON is invalid: %w", err)
	}
	if v, ok := rawObj["variables"]; ok && len(bytes.TrimSpace(v)) > 0 && bytes.TrimSpace(v)[0] == '{' {
		delete(rawObj, "variables")
		cleaned, err := json.Marshal(rawObj)
		if err == nil {
			working = string(cleaned)
		}
	}

	var wf WorkflowPipeline
	if err := json.Unmarshal([]byte(working), &wf); err != nil {
		return nil, nil, fmt.Errorf("resolved workflow JSON is invalid: %w", err)
	}

	// Pretty-print resolved JSON
	pretty, err := json.MarshalIndent(&wf, "", "  ")
	if err != nil {
		pretty = []byte(working)
	}
	return pretty, &wf, nil
}

func inferVariableType(key string) string {
	l := strings.ToLower(key)
	switch {
	case strings.Contains(l, "prompt") || strings.Contains(l, "shots") || strings.Contains(l, "description"):
		return "multiline"
	case strings.Contains(l, "image") || strings.Contains(l, "ref") || strings.Contains(l, "file") || strings.Contains(l, "path"):
		return "file"
	case strings.Contains(l, "seed") || strings.Contains(l, "steps") || strings.Contains(l, "fps") || strings.Contains(l, "duration") || strings.Contains(l, "width") || strings.Contains(l, "height"):
		return "int"
	case strings.Contains(l, "cfg") || strings.Contains(l, "strength") || strings.Contains(l, "denoise"):
		return "float"
	default:
		return "string"
	}
}

func variableSortRank(v WorkflowVariable) int {
	l := strings.ToLower(v.Key)
	switch {
	case strings.Contains(l, "positive") || l == "prompt":
		return 1
	case strings.Contains(l, "negative"):
		return 2
	case v.ValueType == "multiline":
		return 3
	case strings.Contains(l, "model"):
		return 4
	case v.ValueType == "file":
		return 5
	case strings.Contains(l, "aspect") || strings.Contains(l, "camera"):
		return 6
	case strings.Contains(l, "steps") || strings.Contains(l, "duration"):
		return 7
	case strings.Contains(l, "seed"):
		return 8
	default:
		return 20
	}
}

func humanizeWorkflowKey(k string) string {
	parts := strings.FieldsFunc(k, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
