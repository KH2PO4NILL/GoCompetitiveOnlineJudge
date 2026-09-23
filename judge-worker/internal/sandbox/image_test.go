package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

type imageDockerAPI struct {
	fakeDockerAPI
	image    string
	deadline time.Time
	result   client.ImageInspectResult
	err      error
}

func (api *imageDockerAPI) ImageInspect(ctx context.Context, image string, _ ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	api.image = image
	api.deadline, _ = ctx.Deadline()
	return api.result, api.err
}

func TestResolveImageInspectsLocalImageWithTimeout(t *testing.T) {
	api := &imageDockerAPI{}
	api.result.ID = "sha256:" + strings.Repeat("a", 64)
	docker := testDocker(t, api)
	before := time.Now()
	digest, err := docker.ResolveImage(context.Background(), "judge:1.0.0")
	if err != nil || digest != api.result.ID || api.image != "judge:1.0.0" {
		t.Fatalf("image = %q, request = %q, error = %v", digest, api.image, err)
	}
	if api.deadline.Before(before) || api.deadline.After(time.Now().Add(docker.apiTimeout)) {
		t.Fatalf("invalid inspection deadline %v", api.deadline)
	}
	api.err = errors.New("image not found")
	if _, err := docker.ResolveImage(context.Background(), "missing:1.0.0"); !errors.Is(err, api.err) {
		t.Fatalf("ResolveImage() error = %v", err)
	}
}

func TestCreateOptionsAppliesExplicitScratchSpace(t *testing.T) {
	docker := testDocker(t, &fakeDockerAPI{})
	request := validExecuteRequest()
	request.Limits.MemoryBytes = 2 << 30
	request.Limits.TmpfsBytes = 512 << 20
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	options := docker.createOptions(request, nil, "scratch-test")
	if got := options.HostConfig.Tmpfs["/tmp"]; got != "rw,noexec,nosuid,nodev,size=536870912,mode=1777" {
		t.Fatalf("tmpfs = %q", got)
	}
}
