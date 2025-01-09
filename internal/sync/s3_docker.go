package sync

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
)

func extractManifestsAndLayers(ctx context.Context, s3c *s3Client, d partial.Describable, manifests []*manifestWithMediaType, layers []v1.Layer) ([]*manifestWithMediaType, []v1.Layer, error) {
	switch obj := d.(type) {
	case v1.ImageIndex:
		b, err := obj.RawManifest()
		if err != nil {
			return manifests, layers, err
		}
		if !containsManifest(manifests, b) {
			childMediaType, err := obj.MediaType()
			if err != nil {
				return manifests, layers, err
			}
			childDigest, err := obj.Digest()
			if err != nil {
				return manifests, layers, err
			}
			manifests = append(manifests, &manifestWithMediaType{
				Manifest:  b,
				MediaType: string(childMediaType),
				Digest:    childDigest.String(),
			})
		}
	case v1.Image:
		if err := extractConfigFile(ctx, s3c, obj); err != nil {
			return manifests, layers, err
		}

		b, err := obj.RawManifest()
		if err != nil {
			return manifests, layers, err
		}
		if !containsManifest(manifests, b) {
			childMediaType, err := obj.MediaType()
			if err != nil {
				return manifests, layers, err
			}
			childDigest, err := obj.Digest()
			if err != nil {
				return manifests, layers, err
			}
			manifests = append(manifests, &manifestWithMediaType{
				Manifest:  b,
				MediaType: string(childMediaType),
				Digest:    childDigest.String(),
			})
		}
		l, err := obj.Layers()
		if err != nil {
			return manifests, layers, err
		}
		for _, layer := range l {
			layers = appendLayerIfNotExists(layers, layer)
		}
	case v1.Layer:
		layers = appendLayerIfNotExists(layers, obj)
	}
	return manifests, layers, nil
}

func extractConfigFile(ctx context.Context, s3c *s3Client, i v1.Image) error {
	if cnf, err := i.RawConfigFile(); err == nil {
		// Config is optional, so ignore if it's not found.
		if cnfHash, err := i.ConfigName(); err == nil {
			if err := syncObject(
				ctx,
				s3c,
				filepath.Join(s3c.baseDir, "blobs", cnfHash.String()),
				aws.String("application/vnd.oci.image.config.v1+json"),
				bytes.NewReader(cnf),
			); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to get config name: %w", err)
		}
	}

	return nil
}
