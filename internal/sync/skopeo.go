package sync

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func SkopeoCopy(ctx context.Context, src string, srcAuth []string, dest string, destAuth []string) error {
	args := []string{"--insecure-policy", "copy", "-a", "-q"}
	args = append(args, srcAuth...)
	args = append(args, destAuth...)

	if len(strings.Split(src, "/")) == 2 {
		src = fmt.Sprintf("docker.io/%s", src)
	}
	src = fmt.Sprintf("docker://%s", src)

	if len(strings.Split(dest, "/")) == 2 {
		dest = fmt.Sprintf("docker.io/%s", dest)
	}
	dest = fmt.Sprintf("docker://%s", dest)

	args = append(args, src, dest)

	cmd := exec.CommandContext(ctx, "skopeo", args...)

	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("skopeo failed: %v: %s", err, string(b))
	}

	return nil
}
