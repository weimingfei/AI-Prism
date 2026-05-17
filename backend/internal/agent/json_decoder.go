package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var errAgentJSONNotFound = errors.New("agent json object not found")

var agentJSONAliases = map[string]string{
	"mastery_score":       "masteryScore",
	"correct_parts":       "correctParts",
	"missing_parts":       "missingParts",
	"missing_points":      "missingPoints",
	"follow_up":           "followUp",
	"follow_up_needed":    "followUpNeeded",
	"follow_up_question":  "followUpQuestion",
	"ask_to_user":         "askToUser",
	"end_practice":        "endPractice",
	"logic_ok":            "logicOk",
	"markdown_card":       "markdownCard",
	"review_plan":         "reviewPlan",
	"overall_comment":     "overallComment",
	"knowledge_title":     "knowledgeTitle",
	"check_prompt":        "checkPrompt",
	"readable_size":       "readableSize",
	"need_miner_u":        "needMinerU",
	"need_mineru":         "needMinerU",
	"material_scope":      "materialScope",
	"current_round":       "currentRound",
	"max_round":           "maxRound",
	"question_number":     "questionNumber",
	"knowledge_context":   "knowledgeContext",
	"mis_conceptions":     "misconceptions",
	"mis_conception":      "misconceptions",
	"misconception":       "misconceptions",
	"diagnose":            "diagnosis",
	"diagnostics":         "diagnosis",
	"correction_suggest":  "correction",
	"correction_feedback": "correction",
}

func decodeAgentJSON(content string, output any, requiredKeys ...string) error {
	value, err := parseAgentJSONObject(content, requiredKeys...)
	if err != nil {
		return err
	}
	normalized := normalizeAgentJSONValue(value)
	data, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, output)
}

func parseAgentJSONObject(content string, requiredKeys ...string) (map[string]any, error) {
	content = stripAgentMarkdownFence(content)
	if strings.TrimSpace(content) == "" {
		return nil, errAgentJSONNotFound
	}
	candidates := []string{content}
	if extracted := extractFirstJSONObjectByState(content); strings.TrimSpace(extracted) != "" && extracted != content {
		candidates = append(candidates, extracted)
	}
	if start := strings.Index(content, "{"); start >= 0 {
		if end := strings.LastIndex(content, "}"); end > start {
			candidates = append(candidates, content[start:end+1])
		}
	}

	var lastErr error
	for _, candidate := range candidates {
		var decoded any
		decoder := json.NewDecoder(strings.NewReader(candidate))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err != nil {
			lastErr = err
			continue
		}
		normalized := normalizeAgentJSONValue(decoded)
		if unwrapped, ok := unwrapAgentJSONObject(normalized); ok {
			normalized = unwrapped
		}
		if found, ok := findObjectContainingKeys(normalized, requiredKeys...); ok {
			return found, nil
		}
		if object, ok := normalized.(map[string]any); ok {
			return object, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errAgentJSONNotFound
}

func stripAgentMarkdownFence(text string) string {
	cleaned := strings.TrimSpace(text)
	if !strings.HasPrefix(cleaned, "```") {
		return cleaned
	}
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```JSON")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

// 模型偶尔会在 JSON 前后夹解释文字，这里用简单状态机找第一个完整对象。
// 不能只用字符串截取，否则遇到字符串内部的花括号会截错。
func extractFirstJSONObjectByState(text string) string {
	start := -1
	depth := 0
	inString := false
	escaped := false
	for index, r := range text {
		if start < 0 {
			if r == '{' {
				start = index
				depth = 1
			}
			continue
		}
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : index+1]
			}
		}
	}
	return ""
}

func unwrapAgentJSONObject(value any) (any, bool) {
	object, ok := value.(map[string]any)
	if !ok || len(object) == 0 {
		return value, false
	}
	for _, key := range []string{"json", "data", "result", "output", "response"} {
		field, exists := object[key]
		if !exists {
			continue
		}
		switch typed := field.(type) {
		case map[string]any:
			return typed, true
		case string:
			nested, err := parseAgentJSONObject(typed)
			if err == nil {
				return nested, true
			}
		}
	}
	if choices, ok := object["choices"].([]any); ok && len(choices) > 0 {
		if nested, ok := findObjectContainingKeys(choices[0]); ok {
			return nested, true
		}
	}
	return value, false
}

func findObjectContainingKeys(value any, requiredKeys ...string) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if len(requiredKeys) == 0 || containsAnyAgentKey(typed, requiredKeys...) {
			return typed, true
		}
		for _, nested := range typed {
			if found, ok := findObjectContainingKeys(nested, requiredKeys...); ok {
				return found, true
			}
		}
	case []any:
		for _, item := range typed {
			if found, ok := findObjectContainingKeys(item, requiredKeys...); ok {
				return found, true
			}
		}
	case string:
		nested, err := parseAgentJSONObject(typed, requiredKeys...)
		if err == nil {
			return nested, true
		}
	}
	return nil, false
}

func containsAnyAgentKey(object map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := object[key]; ok {
			return true
		}
		if alias, ok := agentJSONAliases[key]; ok {
			if _, exists := object[alias]; exists {
				return true
			}
		}
	}
	return false
}

func normalizeAgentJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalizedKey := normalizeAgentJSONKey(key)
			out[normalizedKey] = normalizeAgentJSONValue(nested)
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			out[normalizeAgentJSONKey(key)] = nested
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, nested := range typed {
			out = append(out, normalizeAgentJSONValue(nested))
		}
		return out
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return i
		}
		if f, err := typed.Float64(); err == nil {
			return f
		}
		return typed.String()
	case string:
		return normalizeScalarString(typed)
	default:
		return normalizeReflectMaps(value)
	}
}

func normalizeAgentJSONKey(key string) string {
	key = strings.TrimSpace(key)
	if alias, ok := agentJSONAliases[key]; ok {
		return alias
	}
	lower := strings.ToLower(key)
	if alias, ok := agentJSONAliases[lower]; ok {
		return alias
	}
	return key
}

func normalizeScalarString(value string) any {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	switch lower {
	case "true", "yes", "y":
		return true
	case "false", "no", "n":
		return false
	case "null", "none", "n/a", "":
		return trimmed
	}
	if i, err := strconv.Atoi(trimmed); err == nil {
		return i
	}
	return value
}

func normalizeReflectMaps(value any) any {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() != reflect.Map {
		return value
	}
	out := map[string]any{}
	iter := rv.MapRange()
	for iter.Next() {
		out[normalizeAgentJSONKey(fmt.Sprint(iter.Key().Interface()))] = normalizeAgentJSONValue(iter.Value().Interface())
	}
	return out
}

func marshalStrictJSONObject(schemaName string, fields map[string]string) string {
	var b bytes.Buffer
	b.WriteString("只输出一个严格 JSON 对象，不要 Markdown，不要解释，不要把 JSON 放进字符串字段。")
	if schemaName != "" {
		b.WriteString("\nSchema: ")
		b.WriteString(schemaName)
	}
	if len(fields) > 0 {
		b.WriteString("\n字段约束：")
		for key, desc := range fields {
			b.WriteString("\n- ")
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(desc)
		}
	}
	return b.String()
}
