package entes

import (
	"bufio"
	"strings"
)

// parseSimpleYAML parses the very limited YAML shape used in entes-federal.yaml:
//
//	entes:
//	  - id: fed:uniao
//	    nome: ...
//	    key: value
//
// It does not support nested objects, arrays of scalars, anchors, multi-line
// strings, or any advanced YAML feature. This is intentional: it avoids
// pulling gopkg.in/yaml.v3 as a dependency for one static file.
//
// Each record is returned as a map of string keys to string values. List
// items are identified by lines starting with "- " at indent 2.
func parseSimpleYAML(input string) ([]map[string]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	// Some lines can be long (unlikely, but set a generous buffer anyway).
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var out []map[string]string
	var current map[string]string
	inEntes := false

	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimRight(raw, " \t")
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)

		if !inEntes {
			if strings.HasPrefix(trimmed, "entes:") {
				inEntes = true
			}
			continue
		}

		// List item start: "- key: value" at indent 2
		if indent == 2 && strings.HasPrefix(trimmed, "- ") {
			if current != nil {
				out = append(out, current)
			}
			current = make(map[string]string)
			rest := strings.TrimPrefix(trimmed, "- ")
			k, v := splitKV(rest)
			if k != "" {
				current[k] = v
			}
			continue
		}

		// Key on an existing list item: indent 4, "key: value"
		if indent >= 4 && current != nil {
			k, v := splitKV(trimmed)
			if k != "" {
				current[k] = v
			}
			continue
		}
	}
	if current != nil {
		out = append(out, current)
	}
	return out, scanner.Err()
}

// splitKV splits "key: value" and trims/unquotes the value.
func splitKV(s string) (string, string) {
	idx := strings.Index(s, ":")
	if idx == -1 {
		return "", ""
	}
	k := strings.TrimSpace(s[:idx])
	v := strings.TrimSpace(s[idx+1:])
	// Remove trailing comments on the same line.
	if hashIdx := strings.Index(v, " #"); hashIdx >= 0 {
		v = strings.TrimSpace(v[:hashIdx])
	}
	// Unquote a quoted string.
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"') {
		v = v[1 : len(v)-1]
	}
	return k, v
}
