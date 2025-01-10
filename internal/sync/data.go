package sync

import (
	"context"
	"io"

	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/containers/image/v5/types"
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

func dockerDataCounter(ctx context.Context, src string, dst string, ch chan types.ProgressProperties) {
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-ch:
			if p.OffsetUpdate > 0 {
				telemetry.DownloadedBytes.Add(ctx, int64(p.OffsetUpdate),
					metric.WithAttributes(
						attribute.KeyValue{
							Key:   "source",
							Value: attribute.StringValue(src),
						},
						attribute.KeyValue{
							Key:   "type",
							Value: attribute.StringValue("docker"),
						},
					),
				)

				if dst != "" {
					telemetry.UploadedBytes.Add(ctx, int64(p.OffsetUpdate),
						metric.WithAttributes(
							attribute.KeyValue{
								Key:   "destination",
								Value: attribute.StringValue(dst),
							},
							attribute.KeyValue{
								Key:   "type",
								Value: attribute.StringValue("docker"),
							},
						),
					)
				}
			}
		}
	}
}
