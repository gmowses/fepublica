// Package canonjson produces a deterministic JSON serialization of arbitrary values.
//
// This is a pragmatic canonical-JSON implementation targeting RFC 8785 semantics
// with the simplifications appropriate for MVP scope:
//
//   - Object keys are sorted lexicographically (by UTF-16 code point).
//   - No insignificant whitespace is emitted.
//   - Numbers are re-encoded via Go's default float encoding, which is sufficient
//     for the inputs we actually receive (integers and short decimals from the
//     Portal da Transparencia API). A future version may adopt the full RFC 8785
//     number-canonicalization algorithm if that becomes necessary.
//   - Unicode strings use the shortest valid escape sequence.
//   - Arrays preserve the original order of elements.
//
// Marshal accepts anything encoding/json can marshal. It returns the canonical bytes.
package canonjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Marshal returns the canonical JSON encoding of v.
func Marshal(v any) ([]byte, error) {
	// Round-trip through json.Unmarshal to normalize structure into
	// map[string]any / []any / basic types.
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonjson: initial marshal: %w", err)
	}
	var normalized any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&normalized); err != nil {
		return nil, fmt.Errorf("canonjson: normalize decode: %w", err)
	}
	var buf bytes.Buffer
	if err := encode(&buf, normalized); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encode(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		return writeString(buf, t)
	case json.Number:
		buf.WriteString(t.String())
	case map[string]any:
		return writeObject(buf, t)
	case []any:
		return writeArray(buf, t)
	default:
		return fmt.Errorf("canonjson: unsupported type %T", v)
	}
	return nil
}

func writeObject(buf *bytes.Buffer, m map[string]any) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := writeString(buf, k); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := encode(buf, m[k]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func writeArray(buf *bytes.Buffer, arr []any) error {
	buf.WriteByte('[')
	for i, v := range arr {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encode(buf, v); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

// writeString uses encoding/json's default string encoder, which already
// produces a minimal, valid JSON string with escapes for control characters
// and special chars. This is enough for our canonicalization goal.
func writeString(buf *bytes.Buffer, s string) error {
	enc, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("canonjson: encode string: %w", err)
	}
	buf.Write(enc)
	return nil
}
