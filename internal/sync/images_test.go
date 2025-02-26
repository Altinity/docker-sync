package sync

import (
	"errors"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
)

func TestCheckRateLimit(t *testing.T) {
	tests := []struct {
		name          string
		inputErr      error
		expectedErr   error
		isPermanent   bool
		shouldContain string
	}{
		{
			name:        "nil error",
			inputErr:    nil,
			expectedErr: nil,
			isPermanent: false,
		},
		{
			name:          "HAP429 rate limit error",
			inputErr:      errors.New("received HAP429 from registry"),
			expectedErr:   errors.New("received HAP429 from registry"),
			isPermanent:   false,
			shouldContain: "HAP429",
		},
		{
			name:          "TOOMANYREQUESTS rate limit error",
			inputErr:      errors.New("TOOMANYREQUESTS: please slow down"),
			expectedErr:   errors.New("TOOMANYREQUESTS: please slow down"),
			isPermanent:   false,
			shouldContain: "TOOMANYREQUESTS",
		},
		{
			name:        "non-rate limit error",
			inputErr:    errors.New("general error"),
			expectedErr: backoff.Permanent(errors.New("general error")),
			isPermanent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkRateLimit(tt.inputErr)

			// Check if error is nil when expected
			if tt.expectedErr == nil {
				assert.Nil(t, result)
				return
			}

			// Assert error is not nil when expected
			assert.NotNil(t, result)

			// Check if error message matches
			assert.Equal(t, tt.expectedErr.Error(), result.Error())

			// If it should be a rate limit error, verify the content
			if tt.shouldContain != "" {
				assert.Contains(t, result.Error(), tt.shouldContain)
			}

			// Check if error is permanent when expected
			if tt.isPermanent {
				// Use type assertion to verify if it's a permanent error
				_, isPerm := result.(*backoff.PermanentError)
				assert.True(t, isPerm, "Expected permanent error")
			} else {
				// Verify it's not a permanent error
				_, isPerm := result.(*backoff.PermanentError)
				assert.False(t, isPerm, "Expected non-permanent error")
			}
		})
	}
}
