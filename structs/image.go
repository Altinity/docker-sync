package structs

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Image struct {
	Source      string                   `json:"source" yaml:"source"`
	Targets     []string                 `json:"targets" yaml:"targets"`
	MutableTags []string                 `json:"mutableTags" yaml:"mutableTags"`
	Auths       map[string]remote.Option `json:"-" yaml:"-"`
}

func (i *Image) GetSource() string {
	return i.Source
}

func (i *Image) GetTargets() []string {
	return i.Targets
}

func (i *Image) GetSourceRegistry() string {
	return i.GetRegistry(i.Source)
}

func (i *Image) GetRegistry(url string) string {
	fields := strings.Split(url, "/")

	if strings.HasPrefix(url, "public.ecr.aws") {
		return strings.Join(fields[:2], "/")
	}

	if len(fields) == 2 {
		return "docker.io"
	}

	return fields[0]
}

func (i *Image) GetSourceRepository() string {
	return i.GetRepository(i.Source)
}

func (i *Image) GetName() string {
	fields := strings.Split(i.Source, "/")

	return fields[len(fields)-1]
}

func (i *Image) GetRepository(url string) string {
	fields := strings.Split(url, "/")

	if strings.HasPrefix(url, "public.ecr.aws") {
		return strings.Join(fields[2:], "/")
	}

	return strings.Join(fields[1:], "/")
}
