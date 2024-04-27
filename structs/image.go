package structs

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Image struct {
	Source            string   `json:"source" yaml:"source"`
	Tags              []string `json:"tags" yaml:"tags"`
	Targets           []string `json:"targets" yaml:"targets"`
	RequiredPlatforms []string `json:"requiredPlatforms" yaml:"requiredPlatforms"`
}

func (i *Image) GetSource() string {
	return i.Source
}

func (i *Image) GetTags() ([]string, error) {
	tags := i.Tags

	if len(tags) == 0 {
		repo, err := name.NewRepository(i.Source)
		if err != nil {
			return tags, err
		}

		return remote.List(repo)
	}

	return tags, nil
}

func (i *Image) GetTargets() []string {
	return i.Targets
}

func (i *Image) GetRequiredPlatforms() []string {
	return i.RequiredPlatforms
}

func (i *Image) GetSourceRegistry() string {
	return i.GetRegistry(i.Source)
}

func (i *Image) GetRegistry(url string) string {
	fields := strings.Split(url, "/")

	if strings.HasPrefix(url, "public.ecr.aws") {
		return strings.Join(fields[:2], "/")
	}

	return fields[0]
}

func (i *Image) GetSourceRepository() string {
	return i.GetRepository(i.Source)
}

func (i *Image) GetRepository(url string) string {
	fields := strings.Split(url, "/")

	if strings.HasPrefix(url, "public.ecr.aws") {
		return strings.Join(fields[2:], "/")
	}

	return strings.Join(fields[1:], "/")
}
