package sync

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/containers/image/v5/types"
	"github.com/stretchr/testify/assert"
)

// MockReader implements io.ReadSeeker for testing
type MockReader struct {
	*strings.Reader
}

func TestS3DataCounter_Read(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		bufferSize  int
		expectedN   int
		expectedErr error
	}{
		{
			name:        "successful read",
			input:       "test data",
			bufferSize:  4,
			expectedN:   4,
			expectedErr: nil,
		},
		{
			name:        "empty input",
			input:       "",
			bufferSize:  4,
			expectedN:   0,
			expectedErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			reader := &MockReader{strings.NewReader(tt.input)}
			counter := s3DataCounter{
				ctx:  ctx,
				f:    reader,
				dest: "test-destination",
			}

			buffer := make([]byte, tt.bufferSize)
			n, err := counter.Read(buffer)

			assert.Equal(t, tt.expectedN, n)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestS3DataCounter_Seek(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		offset      int64
		whence      int
		expectedPos int64
		expectedErr error
	}{
		{
			name:        "seek from start",
			input:       "test data",
			offset:      2,
			whence:      io.SeekStart,
			expectedPos: 2,
			expectedErr: nil,
		},
		{
			name:        "seek from current",
			input:       "test data",
			offset:      1,
			whence:      io.SeekCurrent,
			expectedPos: 1,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			reader := &MockReader{strings.NewReader(tt.input)}
			counter := s3DataCounter{
				ctx:  ctx,
				f:    reader,
				dest: "test-destination",
			}

			pos, err := counter.Seek(tt.offset, tt.whence)

			assert.Equal(t, tt.expectedPos, pos)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestDockerDataCounter(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		dst            string
		progressValues []types.ProgressProperties
		contextTimeout time.Duration
	}{
		{
			name: "basic progress updates",
			src:  "source-image",
			dst:  "dest-image",
			progressValues: []types.ProgressProperties{
				{OffsetUpdate: 100},
				{OffsetUpdate: 200},
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "progress updates without destination",
			src:  "source-image",
			dst:  "",
			progressValues: []types.ProgressProperties{
				{OffsetUpdate: 100},
			},
			contextTimeout: 100 * time.Millisecond,
		},
		{
			name: "zero offset update",
			src:  "source-image",
			dst:  "dest-image",
			progressValues: []types.ProgressProperties{
				{OffsetUpdate: 0},
			},
			contextTimeout: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			ch := make(chan types.ProgressProperties, len(tt.progressValues))

			// Send progress values to channel
			go func() {
				for _, p := range tt.progressValues {
					ch <- p
				}
			}()

			// Run dockerDataCounter in a goroutine
			go dockerDataCounter(ctx, tt.src, tt.dst, ch)

			// Wait for context timeout
			<-ctx.Done()
		})
	}
}
