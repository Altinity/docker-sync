package sync

import (
	"bytes"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func getRepositoryType(dst string) RepositoryType {
	fields := strings.Split(dst, ":")

	// If destination has format <type>:<region/endpoint>:<bucket>:<image>, then it's an S3-compatible storage
	if len(fields) == 4 {
		return S3CompatibleRepository
	}

	return OCIRepository
}

func containsManifest(manifests []*manifestWithMediaType, manifest []byte) bool {
	for _, m := range manifests {
		if bytes.Equal(m.Manifest, manifest) {
			return true
		}
	}
	return false
}

func appendLayerIfNotExists(layers []v1.Layer, layer v1.Layer) []v1.Layer {
	layerDigest, err := layer.Digest()
	if err != nil {
		return layers
	}
	for _, l := range layers {
		ldigest, _ := l.Digest()
		if ldigest == layerDigest {
			return layers
		}
	}
	return append(layers, layer)
}

func manifestKey(repository string, tag string) string {
	return filepath.Join("v2", repository, "manifests", tag)
}
