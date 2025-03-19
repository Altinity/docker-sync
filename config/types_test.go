package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestKey_GetMethods(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		k := &Key{Value: "test"}
		assert.Equal(t, "test", k.String())
	})

	t.Run("Int", func(t *testing.T) {
		k := &Key{Value: 42}
		assert.Equal(t, 42, k.Int())
	})

	t.Run("Int64", func(t *testing.T) {
		k := &Key{Value: int64(42)}
		assert.Equal(t, int64(42), k.Int64())
	})

	t.Run("UInt64", func(t *testing.T) {
		k := &Key{Value: uint64(42)}
		assert.Equal(t, uint64(42), k.UInt64())
	})

	t.Run("Duration", func(t *testing.T) {
		k := &Key{Value: time.Hour}
		assert.Equal(t, time.Hour, k.Duration())
	})

	t.Run("Bool", func(t *testing.T) {
		k := &Key{Value: true}
		assert.True(t, k.Bool())
	})

	t.Run("Float64", func(t *testing.T) {
		k := &Key{Value: 3.14}
		assert.Equal(t, 3.14, k.Float64())
	})

	t.Run("StringSlice", func(t *testing.T) {
		k := &Key{Value: []string{"a", "b", "c"}}
		assert.Equal(t, []string{"a", "b", "c"}, k.StringSlice())
	})
}

func TestKey_Update(t *testing.T) {
	viper.Reset()
	viper.Set("test_key", "old_value")

	k := &Key{
		Name:  "test_key",
		Value: "old_value",
	}

	t.Run("No Change", func(t *testing.T) {
		result := k.Update()
		assert.Nil(t, result)
	})

	t.Run("Value Changed", func(t *testing.T) {
		viper.Set("test_key", "new_value")
		result := k.Update()
		assert.NotNil(t, result)
		assert.Equal(t, "test_key", result.Key)
		assert.Equal(t, "old_value", result.OldValue)
		assert.Equal(t, "new_value", result.NewValue)
		assert.Nil(t, result.Error)
	})

	t.Run("Validation Failure", func(t *testing.T) {
		k.ValidationFuncs = []func(interface{}) error{
			func(v interface{}) error {
				return assert.AnError
			},
		}
		viper.Set("test_key", "another_value")
		result := k.Update()
		assert.NotNil(t, result)
		assert.Equal(t, "test_key", result.Key)
		assert.Equal(t, "new_value", result.OldValue)
		assert.Equal(t, "another_value", result.NewValue)
		assert.Error(t, result.Error)
	})
}

func TestNewKey(t *testing.T) {
	t.Run("Basic Key", func(t *testing.T) {
		k := NewKey("test_key")
		assert.Equal(t, "test_key", k.Name)
		assert.Nil(t, k.Default)
		assert.Empty(t, k.ValidationFuncs)
	})

	t.Run("Key with Default Value", func(t *testing.T) {
		k := NewKey("test_key", WithDefaultValue("default"))
		assert.Equal(t, "test_key", k.Name)
		assert.Equal(t, "default", k.Default)
	})

	t.Run("Key with Validation", func(t *testing.T) {
		k := NewKey("test_key", WithValidString())
		assert.Equal(t, "test_key", k.Name)
		assert.Len(t, k.ValidationFuncs, 1)
	})
}

func TestValidationOptions(t *testing.T) {
	tests := []struct {
		name          string
		option        KeyOption
		validValue    interface{}
		invalidValue  interface{}
		expectedError string
	}{
		{"WithAllowedStrings", WithAllowedStrings([]string{"a", "b"}), "a", "c", "value \"c\" is not allowed"},
		{"WithAllowedInts", WithAllowedInts([]int{1, 2}), 1, 3, "value 3 is not allowed"},
		{"WithValidInt", WithValidInt(), 123, "test", "unable to cast"},
		{"WithValidDuration", WithValidDuration(), "1h", "invalid", "time: invalid duration"},
		{"WithValidBool", WithValidBool(), true, "not_bool", "invalid syntax"},
		{"WithValidFloat64", WithValidFloat64(), 3.14, "not_float", "unable to cast"},
		{"WithValidPositiveInt", WithValidPositiveInt(), 5, -5, "value must be positive"},
		{"WithValidURL", WithValidURL(), "http://example.com", "not_url", "lookup : no such host"},
		{"WithValidURI", WithValidURI(), "/path/to/resource", "://invalid", "invalid URL path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &Key{}
			tt.option(k)
			assert.Len(t, k.ValidationFuncs, 1)

			err := k.ValidationFuncs[0](tt.validValue)
			assert.NoError(t, err)

			err = k.ValidationFuncs[0](tt.invalidValue)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestWithValidExistingPathOrEmpty(t *testing.T) {
	t.Run("Empty Path", func(t *testing.T) {
		k := &Key{}
		WithValidExistingPathOrEmpty()(k)
		err := k.ValidationFuncs[0]("")
		assert.NoError(t, err)
	})

	t.Run("Existing Path", func(t *testing.T) {
		tempDir := t.TempDir()

		k := &Key{}
		WithValidExistingPathOrEmpty()(k)
		err := k.ValidationFuncs[0](tempDir)
		assert.NoError(t, err)
	})

	t.Run("Non-existing Path", func(t *testing.T) {
		k := &Key{}
		WithValidExistingPathOrEmpty()(k)
		err := k.ValidationFuncs[0]("/non/existing/path")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})
}

func TestWithValidMap(t *testing.T) {
	k := &Key{}
	WithValidMap()(k)

	t.Run("Valid Map", func(t *testing.T) {
		validMap := map[string]interface{}{"key": "value"}
		err := k.ValidationFuncs[0](validMap)
		assert.NoError(t, err)
	})

	t.Run("Invalid Map", func(t *testing.T) {
		invalidMap := "not a map"
		err := k.ValidationFuncs[0](invalidMap)
		assert.Error(t, err)
	})
}

func TestWithValidImages(t *testing.T) {
	k := &Key{}
	WithValidImages()(k)

	t.Run("Valid Images", func(t *testing.T) {
		validImages := []map[string]interface{}{
			{"source": "src1", "targets": []string{"target1"}},
			{"source": "src2", "targets": []string{"target2", "target3"}},
		}
		err := k.ValidationFuncs[0](validImages)
		assert.NoError(t, err)
	})

	t.Run("Invalid Images - Missing Source", func(t *testing.T) {
		invalidImages := []map[string]interface{}{
			{"targets": []string{"target1"}},
		}
		err := k.ValidationFuncs[0](invalidImages)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source is required")
	})

	t.Run("Invalid Images - Missing Targets", func(t *testing.T) {
		invalidImages := []map[string]interface{}{
			{"source": "src1"},
		}
		err := k.ValidationFuncs[0](invalidImages)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one target is required")
	})
}

func TestWithValidRepositories(t *testing.T) {
	k := &Key{}
	WithValidRepositories()(k)

	t.Run("Valid Repositories", func(t *testing.T) {
		validRepos := []map[string]interface{}{
			{"name": "repo1", "url": "http://example.com/repo1"},
			{"name": "repo2", "url": "http://example.com/repo2"},
		}
		err := k.ValidationFuncs[0](validRepos)
		assert.NoError(t, err)
	})

	t.Run("Invalid Repositories - Missing Name", func(t *testing.T) {
		invalidRepos := []map[string]interface{}{
			{"url": "http://example.com/repo1"},
		}
		err := k.ValidationFuncs[0](invalidRepos)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("Invalid Repositories - Missing URL", func(t *testing.T) {
		invalidRepos := []map[string]interface{}{
			{"name": "repo1"},
		}
		err := k.ValidationFuncs[0](invalidRepos)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url is required")
	})
}

func TestKey_register(t *testing.T) {
	viper.Reset()
	keys = make(map[string]*Key)

	k := &Key{
		Name:    "test_key",
		Default: "default_value",
	}

	k.register()

	assert.Contains(t, keys, "test_key")
	assert.Equal(t, k, keys["test_key"])
	assert.Equal(t, "default_value", viper.Get("test_key"))
}

func TestNewKey_Integration(t *testing.T) {
	viper.Reset()
	keys = make(map[string]*Key)

	k := NewKey("test_key",
		WithDefaultValue("default"),
		WithValidString(),
		WithAllowedStrings([]string{"default", "custom"}),
	)

	assert.Equal(t, "test_key", k.Name)
	assert.Equal(t, "default", k.Default)
	assert.Len(t, k.ValidationFuncs, 2)
	assert.Contains(t, keys, "test_key")
	assert.Equal(t, "default", viper.Get("test_key"))

	// Test Update with valid value
	viper.Set("test_key", "custom")
	result := k.Update()
	assert.NotNil(t, result)
	assert.Equal(t, "custom", k.Value)

	// Test Update with invalid value
	viper.Set("test_key", "invalid")
	result = k.Update()
	assert.NotNil(t, result)
	assert.Error(t, result.Error)
	assert.Equal(t, "custom", k.Value) // Value should not change due to validation failure
}
