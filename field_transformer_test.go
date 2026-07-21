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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stringToTime is a test-local example FieldTransformer, demonstrating how
// to parse a string field into a time.Time (e.g. for use with
// WithToAPIFieldTransformer). It is intentionally not part of the public
// API: callers with this need should write their own transformer tailored
// to their exact parsing/formatting requirements.
func stringToTime(layout string) FieldTransformer {
	return func(_ string, value any) (any, error) {
		s, ok := value.(string)
		if !ok {
			return nil, ErrNoMatch
		}
		t, err := time.Parse(layout, s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse time value %q using layout %q: %w", s, layout, err)
		}
		return t, nil
	}
}

// timeToString is a test-local example FieldTransformer, demonstrating how
// to normalize a time-valued string field (e.g. for use with
// WithFromAPIFieldTransformer). It is intentionally not part of the public
// API: callers with this need should write their own transformer tailored
// to their exact parsing/formatting requirements.
func timeToString(layout string) FieldTransformer {
	return func(_ string, value any) (any, error) {
		s, ok := value.(string)
		if !ok {
			return nil, ErrNoMatch
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse time value %q: %w", s, err)
		}
		return t.Format(layout), nil
	}
}

func TestStringToTime(t *testing.T) {
	expectedTime := time.Date(2025, 2, 1, 1, 30, 15, 0, time.UTC)

	t.Run("valid RFC3339 string", func(t *testing.T) {
		fn := stringToTime(time.RFC3339)
		result, err := fn("deleteAfterDate", "2025-02-01T01:30:15Z")
		require.NoError(t, err)
		assert.Equal(t, expectedTime, result)
	})

	t.Run("invalid string returns parse error", func(t *testing.T) {
		fn := stringToTime(time.RFC3339)
		result, err := fn("deleteAfterDate", "not-a-date")
		require.Error(t, err)
		assert.Nil(t, result)
		assert.False(t, errors.Is(err, ErrNoMatch), "parse error should not be ErrNoMatch")
	})

	t.Run("non-string value returns ErrNoMatch", func(t *testing.T) {
		fn := stringToTime(time.RFC3339)
		result, err := fn("deleteAfterDate", 42)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrNoMatch))

		result, err = fn("deleteAfterDate", nil)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrNoMatch))
	})
}

func TestTimeToString(t *testing.T) {
	t.Run("idempotent with RFC3339 layout", func(t *testing.T) {
		fn := timeToString(time.RFC3339)
		result, err := fn("deleteAfterDate", "2025-02-01T01:30:15Z")
		require.NoError(t, err)
		assert.Equal(t, "2025-02-01T01:30:15Z", result)
	})

	t.Run("reformats to different layout", func(t *testing.T) {
		fn := timeToString("2006-01-02")
		result, err := fn("deleteAfterDate", "2025-02-01T01:30:15Z")
		require.NoError(t, err)
		assert.Equal(t, "2025-02-01", result)
	})

	t.Run("normalizes fractional seconds RFC3339", func(t *testing.T) {
		fn := timeToString(time.RFC3339)
		result, err := fn("deleteAfterDate", "2025-02-01T01:30:15.123Z")
		require.NoError(t, err)
		assert.Equal(t, "2025-02-01T01:30:15Z", result)
	})

	t.Run("normalizes numeric offset RFC3339", func(t *testing.T) {
		fn := timeToString(time.RFC3339)
		result, err := fn("deleteAfterDate", "2025-02-01T01:30:15+00:00")
		require.NoError(t, err)
		assert.Equal(t, "2025-02-01T01:30:15Z", result)
	})

	t.Run("invalid string returns parse error", func(t *testing.T) {
		fn := timeToString(time.RFC3339)
		result, err := fn("deleteAfterDate", "not-a-date")
		require.Error(t, err)
		assert.Empty(t, result)
		assert.False(t, errors.Is(err, ErrNoMatch), "parse error should not be ErrNoMatch")
	})

	t.Run("non-string value returns ErrNoMatch", func(t *testing.T) {
		fn := timeToString(time.RFC3339)
		result, err := fn("deleteAfterDate", 42)
		require.Error(t, err)
		assert.Empty(t, result)
		assert.True(t, errors.Is(err, ErrNoMatch))

		result, err = fn("deleteAfterDate", nil)
		require.Error(t, err)
		assert.Empty(t, result)
		assert.True(t, errors.Is(err, ErrNoMatch))
	})
}

func TestWithToAPIFieldTransformerMatching(t *testing.T) {
	called := false
	tr := &translator{}
	opt := WithToAPIFieldTransformer("deleteAfterDate", func(fieldPath string, value any) (any, error) {
		called = true
		return "transformed", nil
	})
	opt(tr)

	require.Len(t, tr.toAPITransformers, 1, "expected one transformer registered")
	fn := tr.toAPITransformers[0]

	t.Run("exact match", func(t *testing.T) {
		called = false
		result, err := fn("deleteAfterDate", "some value")
		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, "transformed", result)
	})

	t.Run("nested field does not match", func(t *testing.T) {
		called = false
		result, err := fn("nested.deleteAfterDate", "some value")
		require.Error(t, err)
		assert.False(t, called)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrNoMatch))
	})

	t.Run("different field does not match", func(t *testing.T) {
		called = false
		result, err := fn("otherField", "some value")
		require.Error(t, err)
		assert.False(t, called)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrNoMatch))
	})
}

func TestWithFromAPIFieldTransformerMatching(t *testing.T) {
	called := false
	tr := &translator{}
	opt := WithFromAPIFieldTransformer("deleteAfterDate", func(fieldPath string, value any) (any, error) {
		called = true
		return "transformed", nil
	})
	opt(tr)

	require.Len(t, tr.fromAPITransformers, 1, "expected one transformer registered")
	fn := tr.fromAPITransformers[0]

	t.Run("exact match", func(t *testing.T) {
		called = false
		result, err := fn("deleteAfterDate", "some value")
		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, "transformed", result)
	})

	t.Run("nested field does not match", func(t *testing.T) {
		called = false
		result, err := fn("nested.deleteAfterDate", "some value")
		require.Error(t, err)
		assert.False(t, called)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrNoMatch))
	})

	t.Run("different field does not match", func(t *testing.T) {
		called = false
		result, err := fn("otherField", "some value")
		require.Error(t, err)
		assert.False(t, called)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrNoMatch))
	})
}

func TestApplyFieldTransformersRecursively(t *testing.T) {
	t.Run("no transformers", func(t *testing.T) {
		input := map[string]any{
			"name": "test",
			"nested": map[string]any{
				"key": "value",
			},
			"items": []any{
				map[string]any{"id": 1},
			},
		}
		result, err := applyFieldTransformersRecursively("", input, nil)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("single transformer replaces top-level field", func(t *testing.T) {
		transformer := FieldTransformer(func(fieldPath string, value any) (any, error) {
			if fieldPath == "deleteAfterDate" {
				return "replaced", nil
			}
			return nil, ErrNoMatch
		})
		input := map[string]any{
			"deleteAfterDate": "2025-01-01T00:00:00Z",
			"name":            "test",
		}
		result, err := applyFieldTransformersRecursively("", input, []FieldTransformer{transformer})
		require.NoError(t, err)
		rm, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "replaced", rm["deleteAfterDate"])
		assert.Equal(t, "test", rm["name"])
	})

	t.Run("transformer matches nested field", func(t *testing.T) {
		tr := &translator{}
		opt := WithToAPIFieldTransformer(`obj\.deleteAfterDate`, func(fieldPath string, value any) (any, error) {
			return "transformed-nested", nil
		})
		opt(tr)
		require.Len(t, tr.toAPITransformers, 1)

		input := map[string]any{
			"obj": map[string]any{
				"deleteAfterDate": "2025-01-01T00:00:00Z",
				"other":           "untouched",
			},
		}
		result, err := applyFieldTransformersRecursively("", input, tr.toAPITransformers)
		require.NoError(t, err)
		rm, ok := result.(map[string]any)
		require.True(t, ok)
		obj, ok := rm["obj"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "transformed-nested", obj["deleteAfterDate"])
		assert.Equal(t, "untouched", obj["other"])
	})

	t.Run("transformer matches field inside array of objects", func(t *testing.T) {
		tr := &translator{}
		opt := WithToAPIFieldTransformer(`items\[\]\.deleteAfterDate`, func(fieldPath string, value any) (any, error) {
			return "transformed-item", nil
		})
		opt(tr)
		require.Len(t, tr.toAPITransformers, 1)

		input := map[string]any{
			"items": []any{
				map[string]any{"deleteAfterDate": "2025-01-01T00:00:00Z", "id": 1},
				map[string]any{"deleteAfterDate": "2025-06-01T00:00:00Z", "id": 2},
			},
		}
		result, err := applyFieldTransformersRecursively("", input, tr.toAPITransformers)
		require.NoError(t, err)
		rm, ok := result.(map[string]any)
		require.True(t, ok)
		items, ok := rm["items"].([]any)
		require.True(t, ok)
		require.Len(t, items, 2)
		item0, ok := items[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "transformed-item", item0["deleteAfterDate"])
		assert.Equal(t, 1, item0["id"])
		item1, ok := items[1].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "transformed-item", item1["deleteAfterDate"])
		assert.Equal(t, 2, item1["id"])
	})

	t.Run("first non-ErrNoMatch transformer wins, second not invoked for that field", func(t *testing.T) {
		var secondCalledForField bool

		first := FieldTransformer(func(fieldPath string, value any) (any, error) {
			if fieldPath == "field" {
				return "first-wins", nil
			}
			return nil, ErrNoMatch
		})
		second := FieldTransformer(func(fieldPath string, value any) (any, error) {
			if fieldPath == "field" {
				secondCalledForField = true
			}
			return nil, ErrNoMatch
		})

		input := map[string]any{
			"field": "original",
		}
		result, err := applyFieldTransformersRecursively("", input, []FieldTransformer{first, second})
		require.NoError(t, err)
		rm, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "first-wins", rm["field"])
		assert.False(t, secondCalledForField, "second transformer should not have been called for field path 'field'")
	})

	t.Run("transformer returning real error aborts walk", func(t *testing.T) {
		sentinelErr := errors.New("something went wrong")

		badFn := FieldTransformer(func(fieldPath string, value any) (any, error) {
			if fieldPath == "badField" {
				return nil, sentinelErr
			}
			return nil, ErrNoMatch
		})

		input := map[string]any{
			"goodField": "hello",
			"badField":  "world",
		}
		result, err := applyFieldTransformersRecursively("", input, []FieldTransformer{badFn})
		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, sentinelErr), "error should wrap the sentinel error")
		assert.Contains(t, err.Error(), "badField")
	})
}
