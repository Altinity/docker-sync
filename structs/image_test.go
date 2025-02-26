package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImage_GetSource(t *testing.T) {
	image := &Image{Source: "example.com/repo/image:tag"}
	assert.Equal(t, "example.com/repo/image:tag", image.GetSource())
}

func TestImage_GetTargets(t *testing.T) {
	image := &Image{Targets: []string{"target1", "target2"}}
	assert.Equal(t, []string{"target1", "target2"}, image.GetTargets())
}

func TestImage_GetSourceRegistry(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{"Docker Hub", "ubuntu:latest", "docker.io"},
		{"Custom Registry", "example.com/repo/image:tag", "example.com"},
		{"AWS ECR", "public.ecr.aws/docker/ubuntu:latest", "public.ecr.aws/docker"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := &Image{Source: tt.source}
			assert.Equal(t, tt.expected, image.GetSourceRegistry())
		})
	}
}

func TestImage_GetRegistry(t *testing.T) {
	image := &Image{}
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"Docker Hub", "ubuntu:latest", "docker.io"},
		{"Custom Registry", "example.com/repo/image:tag", "example.com"},
		{"AWS ECR", "public.ecr.aws/docker/ubuntu:latest", "public.ecr.aws/docker"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, image.GetRegistry(tt.url))
		})
	}
}

func TestImage_GetSourceRepository(t *testing.T) {
	image := &Image{Source: "example.com/repo/image:tag"}
	assert.Equal(t, "repo/image:tag", image.GetSourceRepository())
}

func TestImage_GetName(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{"Simple Name", "ubuntu:latest", "ubuntu:latest"},
		{"With Registry", "example.com/repo/image:tag", "image:tag"},
		{"With Multiple Slashes", "example.com/org/repo/image:tag", "image:tag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := &Image{Source: tt.source}
			assert.Equal(t, tt.expected, image.GetName())
		})
	}
}

func TestImage_GetRepository(t *testing.T) {
	image := &Image{}
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"Docker Hub", "ubuntu:latest", "library/ubuntu:latest"},
		{"Custom Registry", "example.com/repo/image:tag", "repo/image:tag"},
		{"AWS ECR", "public.ecr.aws/docker/ubuntu:latest", "ubuntu:latest"},
		{"R2 URL", "r2:account-id:bucket:repo/image:tag", "repo/image:tag"},
		{"S3 URL", "s3:account-id:bucket:repo/image:tag", "repo/image:tag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, image.GetRepository(tt.url))
		})
	}
}
