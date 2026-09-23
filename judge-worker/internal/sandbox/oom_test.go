package sandbox

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type delayedOOMAPI struct {
	fakeDockerAPI
	inspections int
}

func (api *delayedOOMAPI) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	api.inspections++
	return client.ContainerInspectResult{Container: container.InspectResponse{State: &container.State{OOMKilled: api.inspections > 1}}}, nil
}

func TestInspectExitWaitsForDelayedOOMState(t *testing.T) {
	api := &delayedOOMAPI{}
	docker := testDocker(t, api)
	result, err := docker.inspectExit("fast-oom", 137, "")
	if err != nil || !result.Container.State.OOMKilled || api.inspections != 2 {
		t.Fatalf("result = %+v %v", result, err)
	}
	api.inspections = 0
	if _, err := docker.inspectExit("forced-kill", 137, ExitTLE); err != nil || api.inspections != 1 {
		t.Fatalf("forced inspection: %v %d", err, api.inspections)
	}
}
