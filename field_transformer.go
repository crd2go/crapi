// Copyright 2025 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package crapi

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrNoMatch is returned by a FieldTransformer to signal that it does not
// apply to the given field path or value, so the processor should try the
// next registered transformer (if any) instead of treating it as an error.
var ErrNoMatch = errors.New("field transformer: no match")

// FieldTransformer transforms a field value found at fieldPath during
// translation (ToAPI or FromAPI). fieldPath is a dot-separated path from the
// root of the translated object map (e.g. "deleteAfterDate" or
// "foo.bar"), with array traversal represented by a trailing "[]" segment
// (e.g. "items[].deleteAfterDate").
//
// Implementations must return ErrNoMatch (wrapped or not, checked via
// errors.Is) when they don't apply to this path/value, so that other
// registered transformers get a chance to run. Any other non-nil error
// aborts translation.
type FieldTransformer func(fieldPath string, value any) (any, error)

// TranslatorOption configures optional behavior on a translator created via
// NewTranslator or NewPerVersionTranslators.
type TranslatorOption func(*translator)

// WithToAPIFieldTransformer registers fn to run during ToAPI for any field
// path matching fieldPathPattern. fieldPathPattern is compiled as an
// anchored regular expression (^fieldPathPattern$). Multiple transformers
// can be registered; they run in registration order and the first one that
// does not return ErrNoMatch wins for that field.
func WithToAPIFieldTransformer(fieldPathPattern string, fn FieldTransformer) TranslatorOption {
	wrapped := wrapFieldTransformer(fieldPathPattern, fn)
	return func(t *translator) {
		t.toAPITransformers = append(t.toAPITransformers, wrapped)
	}
}

// WithFromAPIFieldTransformer registers fn to run during FromAPI for any
// field path matching fieldPathPattern. See WithToAPIFieldTransformer for
// matching and precedence semantics.
func WithFromAPIFieldTransformer(fieldPathPattern string, fn FieldTransformer) TranslatorOption {
	wrapped := wrapFieldTransformer(fieldPathPattern, fn)
	return func(t *translator) {
		t.fromAPITransformers = append(t.fromAPITransformers, wrapped)
	}
}

func wrapFieldTransformer(fieldPathPattern string, fn FieldTransformer) FieldTransformer {
	re := regexp.MustCompile("^" + fieldPathPattern + "$")
	return func(fieldPath string, value any) (any, error) {
		if !re.MatchString(fieldPath) {
			return nil, ErrNoMatch
		}
		return fn(fieldPath, value)
	}
}

// applyFieldTransformersRecursively recursively walks value (which may be a
// map[string]any, a []any, or a leaf value), applying the given
// transformers only at leaf values (i.e. values that are not
// map[string]any and not []any). path is the dot-separated field path of
// value itself ("" for the root). For map[string]any values, it recurses
// into every key, extending the path with ".key". For []any values, it
// recurses into every element, extending the path with "[]". For any other
// (leaf) value, it tries each transformer in order: the first one that
// returns a nil error "claims" the leaf and its result is adopted; if every
// transformer returns an error satisfying errors.Is(err, ErrNoMatch), the
// leaf is left unchanged. Any other error aborts the walk.
func applyFieldTransformersRecursively(path string, value any, transformers []FieldTransformer) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			childPath := k
			if path != "" {
				childPath = path + "." + k
			}
			transformed, err := applyFieldTransformersRecursively(childPath, child, transformers)
			if err != nil {
				return nil, err
			}
			v[k] = transformed
		}
		return v, nil
	case []any:
		for i, item := range v {
			transformed, err := applyFieldTransformersRecursively(path+"[]", item, transformers)
			if err != nil {
				return nil, err
			}
			v[i] = transformed
		}
		return v, nil
	default:
		for _, fn := range transformers {
			newValue, err := fn(path, value)
			if err == nil {
				return newValue, nil
			}
			if !errors.Is(err, ErrNoMatch) {
				return nil, fmt.Errorf("field transformer failed at %q: %w", path, err)
			}
		}
		return value, nil
	}
}
