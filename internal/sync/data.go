package sync

import (
	"context"
	"io"

	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type s3DataCounter struct {
	ctx  context.Context
	f    io.ReadSeeker
	dest string
}

func (s s3DataCounter) Read(p []byte) (int, error) {
	n, err := s.f.Read(p)

	if err == nil && n > 0 {
		telemetry.UploadedBytes.Add(s.ctx, int64(n),
			metric.WithAttributes(
				attribute.KeyValue{
					Key:   "destination",
					Value: attribute.StringValue(s.dest),
				},
				attribute.KeyValue{
					Key:   "type",
					Value: attribute.StringValue("s3"),
				},
			),
		)
	}

	return n, err
}

func (s s3DataCounter) Seek(offset int64, whence int) (int64, error) {
	return s.f.Seek(offset, whence)
}

type ociDataCounter struct {
	ctx  context.Context
	f    remote.Taggable
	dest string
}

func (o ociDataCounter) RawManifest() ([]byte, error) {
	b, err := o.f.RawManifest()

	if err == nil && len(b) > 0 {
		telemetry.UploadedBytes.Add(o.ctx, int64(len(b)),
			metric.WithAttributes(
				attribute.KeyValue{
					Key:   "destination",
					Value: attribute.StringValue(o.dest),
				},
				attribute.KeyValue{
					Key:   "type",
					Value: attribute.StringValue("oci"),
				},
			),
		)
	}

	return b, err
}
