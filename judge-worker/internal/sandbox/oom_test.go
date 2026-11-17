package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type delayedOOMAPI struct {
	fakeDockerAPI
	inspections int
	readyAfter  int
}

func (api *delayedOOMAPI) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	api.inspections++
	return client.ContainerInspectResult{Container: container.InspectResponse{State: &container.State{OOMKilled: api.inspections > max(api.readyAfter, 1)}}}, nil
}

func TestInspectExitWaitsForDelayedOOMState(t *testing.T) {
	api := &delayedOOMAPI{}
	docker := testDocker(t, api)
	result, err := docker.inspectExit("fast-oom", 137, "", false)
	if err != nil || !result.Container.State.OOMKilled || api.inspections != 2 {
		t.Fatalf("result = %+v %v", result, err)
	}
	api.inspections = 0
	if _, err := docker.inspectExit("forced-kill", 137, ExitTLE, false); err != nil || api.inspections != 1 {
		t.Fatalf("forced inspection: %v %d", err, api.inspections)
	}
}

func TestInspectExitWaitsBeyondImmediateDockerOOMEvent(t *testing.T) {
	api := &delayedOOMAPI{readyAfter: 15}
	docker := testDocker(t, api)
	docker.apiTimeout = 5 * time.Second
	result, err := docker.inspectExit("slow-oom-event", 137, "", false)
	if err != nil || !result.Container.State.OOMKilled || api.inspections != 16 {
		t.Fatalf("delayed OOM state lost: inspections=%d err=%v", api.inspections, err)
	}
	api.inspections = 0
	if _, err := docker.inspectExit("trusted-supervisor-exit", 137, "", true); err != nil || api.inspections != 1 {
		t.Fatalf("trusted accounting unnecessarily delayed: %d %v", api.inspections, err)
	}
}
