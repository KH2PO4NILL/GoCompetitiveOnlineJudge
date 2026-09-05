package sandbox

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

type fakeDockerAPI struct {
	pingResult client.PingResult
	pingErr    error
	listResult client.ContainerListResult
	listErr    error
	listOpts   client.ContainerListOptions
	killIDs    []string
	removeIDs  []string
	removeOpts []client.ContainerRemoveOptions
	removeErr  map[string]error
	closed     bool
}

func (f *fakeDockerAPI) Ping(context.Context, client.PingOptions) (client.PingResult, error) {
	return f.pingResult, f.pingErr
}
func (f *fakeDockerAPI) ContainerCreate(context.Context, client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	panic("unexpected ContainerCreate")
}
func (f *fakeDockerAPI) ContainerAttach(context.Context, string, client.ContainerAttachOptions) (client.ContainerAttachResult, error) {
	panic("unexpected ContainerAttach")
}
func (f *fakeDockerAPI) ContainerWait(context.Context, string, client.ContainerWaitOptions) client.ContainerWaitResult {
	panic("unexpected ContainerWait")
}
func (f *fakeDockerAPI) ContainerStart(context.Context, string, client.ContainerStartOptions) (client.ContainerStartResult, error) {
	panic("unexpected ContainerStart")
}
func (f *fakeDockerAPI) ContainerStats(context.Context, string, client.ContainerStatsOptions) (client.ContainerStatsResult, error) {
	return client.ContainerStatsResult{Body: io.NopCloser(strings.NewReader("{}"))}, nil
}
func (f *fakeDockerAPI) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	panic("unexpected ContainerInspect")
}
func (f *fakeDockerAPI) ContainerKill(_ context.Context, id string, _ client.ContainerKillOptions) (client.ContainerKillResult, error) {
	f.killIDs = append(f.killIDs, id)
	return client.ContainerKillResult{}, nil
}
func (f *fakeDockerAPI) ContainerRemove(_ context.Context, id string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	f.removeIDs = append(f.removeIDs, id)
	f.removeOpts = append(f.removeOpts, options)
	return client.ContainerRemoveResult{}, f.removeErr[id]
}
func (f *fakeDockerAPI) ContainerList(_ context.Context, options client.ContainerListOptions) (client.ContainerListResult, error) {
	f.listOpts = options
	return f.listResult, f.listErr
}
func (f *fakeDockerAPI) Close() error {
	f.closed = true
	return nil
}

type executeDockerAPI struct {
	fakeDockerAPI
	createOptions client.ContainerCreateOptions
	create        func(context.Context, client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	attachResult  client.ContainerAttachResult
	waitResult    client.ContainerWaitResult
	waitOptions   client.ContainerWaitOptions
	inspectResult client.ContainerInspectResult
	start         func()
	startErr      error
	kill          func()
	stats         func(context.Context, client.ContainerStatsOptions) (client.ContainerStatsResult, error)
}

func (f *executeDockerAPI) ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	f.createOptions = options
	if f.create != nil {
		return f.create(ctx, options)
	}
	return client.ContainerCreateResult{ID: "container-1"}, nil
}
func (f *executeDockerAPI) ContainerAttach(context.Context, string, client.ContainerAttachOptions) (client.ContainerAttachResult, error) {
	return f.attachResult, nil
}
func (f *executeDockerAPI) ContainerWait(_ context.Context, _ string, options client.ContainerWaitOptions) client.ContainerWaitResult {
	f.waitOptions = options
	return f.waitResult
}
func (f *executeDockerAPI) ContainerStart(context.Context, string, client.ContainerStartOptions) (client.ContainerStartResult, error) {
	if f.start != nil {
		f.start()
	}
	return client.ContainerStartResult{}, f.startErr
}
func (f *executeDockerAPI) ContainerStats(ctx context.Context, _ string, options client.ContainerStatsOptions) (client.ContainerStatsResult, error) {
	if f.stats != nil {
		return f.stats(ctx, options)
	}
	return client.ContainerStatsResult{Body: io.NopCloser(strings.NewReader("{}"))}, nil
}
func (f *executeDockerAPI) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return f.inspectResult, nil
}
func (f *executeDockerAPI) ContainerKill(_ context.Context, id string, _ client.ContainerKillOptions) (client.ContainerKillResult, error) {
	f.killIDs = append(f.killIDs, id)
	if f.kill != nil {
		f.kill()
	}
	return client.ContainerKillResult{}, nil
}

func TestExecuteCreateCancellationPreservesContextError(t *testing.T) {
	createStarted := make(chan struct{})
	api := &executeDockerAPI{
		create: func(ctx context.Context, _ client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
			close(createStarted)
			<-ctx.Done()
			return client.ContainerCreateResult{}, ctx.Err()
		},
	}
	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-createStarted
		cancel()
	}()

	result, err := docker.Execute(ctx, request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
	if result.Reason != "" || result.Diagnostic == "" || result.ContainerID != "" {
		t.Fatalf("cancellation result = reason %q diagnostic %q container %q", result.Reason, result.Diagnostic, result.ContainerID)
	}
	if len(api.removeIDs) != 1 || !strings.HasPrefix(api.removeIDs[0], "gcoj-worker-1-attempt-1-") {
		t.Fatalf("ambiguous create cleanup targets = %#v", api.removeIDs)
	}
}

func TestExecuteCreateAPITimeoutIsSandboxError(t *testing.T) {
	api := &executeDockerAPI{
		create: func(ctx context.Context, _ client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
			<-ctx.Done()
			return client.ContainerCreateResult{}, ctx.Err()
		},
	}
	docker := testDocker(t, api)
	docker.apiTimeout = 10 * time.Millisecond
	request := validExecuteRequest()
	request.Mounts = nil

	result, err := docker.Execute(context.Background(), request)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Execute() error = %v, want context.DeadlineExceeded", err)
	}
	if result.Reason != ExitSandboxError || result.Diagnostic == "" {
		t.Fatalf("API timeout result = reason %q diagnostic %q", result.Reason, result.Diagnostic)
	}
}

func TestExecuteCollectsOutputAndExitMetadata(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	resultCh := make(chan container.WaitResponse, 1)
	errorCh := make(chan error, 1)
	api := &executeDockerAPI{
		attachResult: client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
			Conn: clientConn, Reader: bufio.NewReader(clientConn),
		}},
		waitResult: client.ContainerWaitResult{Result: resultCh, Error: errorCh},
		inspectResult: client.ContainerInspectResult{Container: container.InspectResponse{
			State: &container.State{ExitCode: 7},
		}},
	}
	api.start = func() {
		go func() {
			_ = writeDockerFrame(serverConn, stdcopy.Stdout, []byte("out"))
			_ = writeDockerFrame(serverConn, stdcopy.Stderr, []byte("err"))
			_ = serverConn.Close()
			resultCh <- container.WaitResponse{StatusCode: 7}
		}()
	}
	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	result, err := docker.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Reason != ExitNormal || result.ExitCode == nil || *result.ExitCode != 7 {
		t.Fatalf("result termination = reason %q exit %#v", result.Reason, result.ExitCode)
	}
	if string(result.Stdout) != "out" || string(result.Stderr) != "err" || result.OutputBytes != 6 {
		t.Fatalf("result output = stdout %q stderr %q bytes %d", result.Stdout, result.Stderr, result.OutputBytes)
	}
	if result.ContainerID != "container-1" || !slices.Equal(api.removeIDs, []string{"container-1"}) {
		t.Fatalf("container lifecycle = ID %q removed %#v", result.ContainerID, api.removeIDs)
	}
	if api.waitOptions.Condition != container.WaitConditionNextExit {
		t.Fatalf("wait condition = %q, want %q", api.waitOptions.Condition, container.WaitConditionNextExit)
	}
}

func TestExecuteOpensStatsBeforeStartAndCapturesShortExecutionUsage(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	resultCh := make(chan container.WaitResponse, 1)
	errorCh := make(chan error, 1)
	streamReader, streamWriter := io.Pipe()
	defer streamWriter.Close()
	var streamOpened atomic.Bool
	api := &executeDockerAPI{
		attachResult: client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
			Conn: clientConn, Reader: bufio.NewReader(clientConn),
		}},
		waitResult: client.ContainerWaitResult{Result: resultCh, Error: errorCh},
		inspectResult: client.ContainerInspectResult{Container: container.InspectResponse{
			State: &container.State{ExitCode: 0},
		}},
		stats: func(_ context.Context, options client.ContainerStatsOptions) (client.ContainerStatsResult, error) {
			if options.Stream {
				streamOpened.Store(true)
				return client.ContainerStatsResult{Body: streamReader}, nil
			}
			return statsResult(0, 0, 0), nil
		},
	}
	api.start = func() {
		if !streamOpened.Load() {
			t.Fatal("resource statistics stream was not opened before container start")
		}
		go func() {
			_ = json.NewEncoder(streamWriter).Encode(container.StatsResponse{
				CPUStats:    container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: uint64(25 * time.Millisecond)}},
				MemoryStats: container.MemoryStats{Usage: 4096, MaxUsage: 8192},
			})
			_ = streamWriter.Close()
			_ = serverConn.Close()
			resultCh <- container.WaitResponse{StatusCode: 0}
		}()
	}

	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	result, err := docker.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Reason != ExitNormal || result.CPUTime != 25*time.Millisecond || result.PeakMemory != 8192 {
		t.Fatalf("resource result = reason %q CPU %s peak %d", result.Reason, result.CPUTime, result.PeakMemory)
	}
}

func TestExecutePollingEnforcesCPUTimeLimit(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	resultCh := make(chan container.WaitResponse, 1)
	errorCh := make(chan error, 1)
	streamReader, streamWriter := io.Pipe()
	defer streamWriter.Close()
	limit := 100 * time.Millisecond
	api := &executeDockerAPI{
		attachResult: client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
			Conn: clientConn, Reader: bufio.NewReader(clientConn),
		}},
		waitResult: client.ContainerWaitResult{Result: resultCh, Error: errorCh},
		inspectResult: client.ContainerInspectResult{Container: container.InspectResponse{
			State: &container.State{ExitCode: 137},
		}},
		stats: func(_ context.Context, options client.ContainerStatsOptions) (client.ContainerStatsResult, error) {
			if options.Stream {
				return client.ContainerStatsResult{Body: streamReader}, nil
			}
			return statsResult(uint64(limit+10*time.Millisecond), 2048, 4096), nil
		},
	}
	api.kill = func() {
		_ = serverConn.Close()
		resultCh <- container.WaitResponse{StatusCode: 137}
	}

	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	request.Limits.CPUTime = limit
	result, err := docker.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Reason != ExitTLE || result.CPUTime < limit || !slices.Equal(api.killIDs, []string{"container-1"}) {
		t.Fatalf("CPU enforcement result = %#v, kills=%#v", result, api.killIDs)
	}
}

func TestExecuteCountsContainerStartLatencyTowardWallTime(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	resultCh := make(chan container.WaitResponse, 1)
	errorCh := make(chan error, 1)
	api := &executeDockerAPI{
		attachResult: client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
			Conn: clientConn, Reader: bufio.NewReader(clientConn),
		}},
		waitResult: client.ContainerWaitResult{Result: resultCh, Error: errorCh},
		inspectResult: client.ContainerInspectResult{Container: container.InspectResponse{
			State: &container.State{ExitCode: 0},
		}},
	}
	api.start = func() {
		time.Sleep(20 * time.Millisecond)
		_ = serverConn.Close()
		resultCh <- container.WaitResponse{StatusCode: 0}
	}

	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	request.Limits.CPUTime = 5 * time.Millisecond
	request.Limits.WallTime = 5 * time.Millisecond
	result, err := docker.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Reason != ExitTLE || result.WallTime < request.Limits.WallTime {
		t.Fatalf("start latency result = reason %q wall %s", result.Reason, result.WallTime)
	}
	if !slices.Equal(api.killIDs, []string{"container-1"}) {
		t.Fatalf("start latency kills = %#v", api.killIDs)
	}
}

func TestExecuteStartFailureStopsResourceMonitors(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	streamBody := newBlockingReadCloser()
	pollStarted := make(chan struct{})
	pollFinished := make(chan struct{})
	var pollStart sync.Once
	var pollFinish sync.Once
	api := &executeDockerAPI{
		attachResult: client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
			Conn: clientConn, Reader: bufio.NewReader(clientConn),
		}},
		waitResult: client.ContainerWaitResult{Result: make(chan container.WaitResponse), Error: make(chan error)},
		startErr:   errors.New("start failed"),
		stats: func(ctx context.Context, options client.ContainerStatsOptions) (client.ContainerStatsResult, error) {
			if options.Stream {
				return client.ContainerStatsResult{Body: streamBody}, nil
			}
			pollStart.Do(func() { close(pollStarted) })
			<-ctx.Done()
			pollFinish.Do(func() { close(pollFinished) })
			return client.ContainerStatsResult{}, ctx.Err()
		},
	}
	api.start = func() {
		<-streamBody.started
		<-pollStarted
	}

	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	result, err := docker.Execute(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Reason != ExitSandboxError {
		t.Fatalf("result reason = %q", result.Reason)
	}
	select {
	case <-streamBody.finished:
	default:
		t.Fatal("stream monitor was not stopped before Execute returned")
	}
	select {
	case <-pollFinished:
	default:
		t.Fatal("poll monitor was not stopped before Execute returned")
	}
}

func TestExecuteClosesBlockingStdinAfterNormalExit(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	resultCh := make(chan container.WaitResponse, 1)
	errorCh := make(chan error, 1)
	stdin := newBlockingReadCloser()
	api := &executeDockerAPI{
		attachResult: client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
			Conn: clientConn, Reader: bufio.NewReader(clientConn),
		}},
		waitResult: client.ContainerWaitResult{Result: resultCh, Error: errorCh},
		inspectResult: client.ContainerInspectResult{Container: container.InspectResponse{
			State: &container.State{ExitCode: 0},
		}},
	}
	api.start = func() {
		go func() {
			<-stdin.started
			_ = serverConn.Close()
			resultCh <- container.WaitResponse{StatusCode: 0}
		}()
	}

	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	request.Stdin = stdin
	result, err := docker.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Reason != ExitNormal {
		t.Fatalf("result reason = %q", result.Reason)
	}
	select {
	case <-stdin.closed:
	default:
		t.Fatal("blocking stdin was not closed")
	}
	select {
	case <-stdin.finished:
	default:
		t.Fatal("stdin copy goroutine did not finish")
	}
}

func TestResourcePollIntervalIsBounded(t *testing.T) {
	for _, test := range []struct {
		limit time.Duration
		want  time.Duration
	}{
		{limit: time.Millisecond, want: MinResourcePoll},
		{limit: 400 * time.Millisecond, want: 20 * time.Millisecond},
		{limit: 10 * time.Second, want: MaxResourcePoll},
	} {
		if got := resourcePollInterval(test.limit); got != test.want {
			t.Errorf("resourcePollInterval(%s) = %s, want %s", test.limit, got, test.want)
		}
	}
}

func statsResult(cpu, usage, peak uint64) client.ContainerStatsResult {
	encoded, err := json.Marshal(container.StatsResponse{
		CPUStats:    container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: cpu}},
		MemoryStats: container.MemoryStats{Usage: usage, MaxUsage: peak},
	})
	if err != nil {
		panic(err)
	}
	return client.ContainerStatsResult{Body: io.NopCloser(strings.NewReader(string(encoded)))}
}

type blockingReadCloser struct {
	started  chan struct{}
	closed   chan struct{}
	finished chan struct{}
	start    sync.Once
	close    sync.Once
	finish   sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{
		started: make(chan struct{}), closed: make(chan struct{}), finished: make(chan struct{}),
	}
}

func (reader *blockingReadCloser) Read([]byte) (int, error) {
	reader.start.Do(func() { close(reader.started) })
	<-reader.closed
	reader.finish.Do(func() { close(reader.finished) })
	return 0, io.EOF
}

func (reader *blockingReadCloser) Close() error {
	reader.close.Do(func() { close(reader.closed) })
	return nil
}

func writeDockerFrame(writer io.Writer, stream stdcopy.StdType, payload []byte) error {
	var header [8]byte
	header[0] = byte(stream)
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

func TestExecuteCancellationKillsWaitsAndRemovesContainer(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	resultCh := make(chan container.WaitResponse, 1)
	errorCh := make(chan error, 1)
	api := &executeDockerAPI{
		attachResult: client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
			Conn: clientConn, Reader: bufio.NewReader(clientConn),
		}},
		waitResult: client.ContainerWaitResult{Result: resultCh, Error: errorCh},
		inspectResult: client.ContainerInspectResult{Container: container.InspectResponse{
			State: &container.State{ExitCode: 137},
		}},
	}
	api.kill = func() {
		_ = serverConn.Close()
		resultCh <- container.WaitResponse{StatusCode: 137}
	}
	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	result, err := docker.Execute(ctx, request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
	if result.Diagnostic == "" || !slices.Equal(api.killIDs, []string{"container-1"}) || !slices.Equal(api.removeIDs, []string{"container-1"}) {
		t.Fatalf("cancellation result = diagnostic %q kills %#v removes %#v", result.Diagnostic, api.killIDs, api.removeIDs)
	}
}

func TestNewDockerRejectsUnsafeEndpoints(t *testing.T) {
	root := t.TempDir()
	base := DockerOptions{
		WorkerID: "worker-1", User: "65532:65532",
		AllowedRoots: []AllowedRoot{{Path: root}},
	}
	for _, endpoint := range []string{
		"unix:///var/run/docker.sock",
		"unix:///run/docker.sock",
		"tcp://127.0.0.1:2375",
		"http://127.0.0.1:2375",
		"https://127.0.0.1:2376/version",
	} {
		t.Run(endpoint, func(t *testing.T) {
			options := base
			options.Endpoint = endpoint
			if _, err := newDockerWithClient(options, &fakeDockerAPI{}); err == nil {
				t.Fatalf("unsafe endpoint %q was accepted", endpoint)
			}
		})
	}

	for _, options := range []DockerOptions{
		{Endpoint: "unix:///var/run/gcoj-docker-proxy.sock"},
		{Endpoint: "https://docker.internal:2376"},
		{Endpoint: "unix:///var/run/docker.sock", AllowUnrestrictedDaemon: true},
	} {
		options.WorkerID = base.WorkerID
		options.User = base.User
		options.AllowedRoots = base.AllowedRoots
		if docker, err := newDockerWithClient(options, &fakeDockerAPI{}); err != nil {
			t.Errorf("safe/explicit endpoint %q rejected: %v", options.Endpoint, err)
		} else {
			_ = docker.Close()
		}
	}
}

func TestNewDockerValidatesSecurityInputs(t *testing.T) {
	root := t.TempDir()
	for _, user := range []string{"", "root", "0", "00", "0:1000", "user", "1000:", "1000:group"} {
		t.Run(user, func(t *testing.T) {
			_, err := newDockerWithClient(DockerOptions{
				Endpoint: "unix:///var/run/docker.sock", AllowUnrestrictedDaemon: true, WorkerID: "worker-1", User: user,
				AllowedRoots: []AllowedRoot{{Path: root}},
			}, &fakeDockerAPI{})
			if err == nil {
				t.Fatalf("user %q was accepted", user)
			}
		})
	}
	if _, err := newDockerWithClient(DockerOptions{
		Endpoint: "unix:///var/run/docker.sock", AllowUnrestrictedDaemon: true, WorkerID: "worker-1", User: "65532:65532",
		AllowedRoots: []AllowedRoot{{Path: root}}, NanoCPUs: 64*DefaultNanoCPUs + 1,
	}, &fakeDockerAPI{}); err == nil {
		t.Fatal("excessive NanoCPUs was accepted")
	}
}

func TestCreateOptionsAppliesCommandLimitsLabelsAndIsolation(t *testing.T) {
	docker := &Docker{
		workerID: "worker-1", user: "65532:65532", nanoCPUs: DefaultNanoCPUs,
		stopTimeout: 5 * time.Second, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	request := validExecuteRequest()
	request.Stdin = io.NopCloser(strings.NewReader("input"))
	binds := []mount.Mount{{Type: mount.TypeBind, Source: "/host/data", Target: "/sandbox/data", ReadOnly: true}}
	options := docker.createOptions(request, binds, "gcoj-worker-1-attempt-1-random")

	if options.Name == "" || options.Config == nil || options.HostConfig == nil {
		t.Fatalf("incomplete create options: %#v", options)
	}
	if !slices.Equal(options.Config.Entrypoint, []string{request.Program}) || !slices.Equal(options.Config.Cmd, request.Args) {
		t.Fatalf("command = entrypoint %#v cmd %#v", options.Config.Entrypoint, options.Config.Cmd)
	}
	if options.Config.User != docker.user || options.Config.Tty || !options.Config.NetworkDisabled {
		t.Fatalf("container config isolation = user %q tty %v network-disabled %v", options.Config.User, options.Config.Tty, options.Config.NetworkDisabled)
	}
	for key, want := range map[string]string{
		managedLabel: "true", componentLabel: componentValue, workerLabel: docker.workerID,
		attemptLabel: "1", submissionLabel: "2",
	} {
		if got := options.Config.Labels[key]; got != want {
			t.Errorf("label %s = %q, want %q", key, got, want)
		}
	}
	host := options.HostConfig
	if string(host.NetworkMode) != "none" || host.Privileged || !host.ReadonlyRootfs || host.AutoRemove {
		t.Errorf("host isolation = network %q privileged %v readonly %v autoremove %v", host.NetworkMode, host.Privileged, host.ReadonlyRootfs, host.AutoRemove)
	}
	if !slices.Equal([]string(host.CapDrop), []string{"ALL"}) || !slices.Contains(host.SecurityOpt, "no-new-privileges:true") {
		t.Errorf("capabilities/security options = %#v %#v", host.CapDrop, host.SecurityOpt)
	}
	if host.RestartPolicy.Name != container.RestartPolicyDisabled || host.LogConfig.Type != "none" || host.Init == nil || !*host.Init {
		t.Errorf("lifecycle/log/init options = %#v %#v %#v", host.RestartPolicy, host.LogConfig, host.Init)
	}
	if host.Memory != request.Limits.MemoryBytes || host.MemorySwap != request.Limits.MemoryBytes || host.PidsLimit == nil || *host.PidsLimit != request.Limits.PIDs || host.NanoCPUs != docker.nanoCPUs {
		t.Errorf("resource limits = memory %d swap %d pids %#v cpu %d", host.Memory, host.MemorySwap, host.PidsLimit, host.NanoCPUs)
	}
	ulimits := make(map[string][2]int64)
	for _, limit := range host.Ulimits {
		ulimits[limit.Name] = [2]int64{limit.Soft, limit.Hard}
	}
	if ulimits["nofile"] != [2]int64{int64(request.Limits.OpenFiles), int64(request.Limits.OpenFiles)} || ulimits["nproc"] != [2]int64{request.Limits.PIDs, request.Limits.PIDs} {
		t.Errorf("ulimits = %#v", ulimits)
	}
	if len(host.Mounts) != 1 || host.Mounts[0].Target != "/sandbox/data" || !host.Mounts[0].ReadOnly {
		t.Errorf("mounts = %#v", host.Mounts)
	}
	if options.Config.Entrypoint[0] == "sh" || slices.Contains(options.Config.Cmd, "-c") && options.Config.Entrypoint[0] == "sh" {
		t.Fatal("create options introduced a command shell")
	}
}

func TestExecuteReportsMemoryLimitAsPeakAfterOOM(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	resultCh := make(chan container.WaitResponse, 1)
	errorCh := make(chan error, 1)
	api := &executeDockerAPI{
		attachResult: client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
			Conn: clientConn, Reader: bufio.NewReader(clientConn),
		}},
		waitResult: client.ContainerWaitResult{Result: resultCh, Error: errorCh},
		inspectResult: client.ContainerInspectResult{Container: container.InspectResponse{
			State: &container.State{ExitCode: 137, OOMKilled: true},
		}},
	}
	api.start = func() {
		go func() {
			_ = serverConn.Close()
			resultCh <- container.WaitResponse{StatusCode: 137}
		}()
	}

	docker := testDocker(t, api)
	request := validExecuteRequest()
	request.Mounts = nil
	result, err := docker.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Reason != ExitMLE || result.PeakMemory != request.Limits.MemoryBytes {
		t.Fatalf("OOM result = reason %q peak %d, want %d", result.Reason, result.PeakMemory, request.Limits.MemoryBytes)
	}
}

func TestClassifyResult(t *testing.T) {
	tests := []struct {
		name       string
		forced     ExitReason
		status     int64
		state      *container.State
		wantReason ExitReason
		wantSignal string
	}{
		{name: "success", status: 0, state: &container.State{}, wantReason: ExitNormal},
		{name: "nonzero", status: 1, state: &container.State{}, wantReason: ExitNormal},
		{name: "signal", status: 143, state: &container.State{}, wantReason: ExitSignal, wantSignal: "SIGTERM"},
		{name: "non-signal high exit", status: 255, state: &container.State{}, wantReason: ExitNormal},
		{name: "oom before signal", status: 137, state: &container.State{OOMKilled: true}, wantReason: ExitMLE, wantSignal: "SIGKILL"},
		{name: "forced output limit", forced: ExitOLE, status: 137, state: &container.State{}, wantReason: ExitOLE, wantSignal: "SIGKILL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := classifyResult(ExecuteResult{}, test.forced, test.status, test.state)
			if result.Reason != test.wantReason || result.Signal != test.wantSignal || result.ExitCode == nil || *result.ExitCode != int(test.status) {
				t.Fatalf("classifyResult() = reason %q signal %q exit %#v", result.Reason, result.Signal, result.ExitCode)
			}
		})
	}
}

func TestCleanupUsesWorkerLabelsAndRemovesOnlyListedContainers(t *testing.T) {
	api := &fakeDockerAPI{
		pingResult: client.PingResult{OSType: "linux"},
		listResult: client.ContainerListResult{Items: []container.Summary{
			{ID: "running", State: container.StateRunning},
			{ID: "exited", State: container.StateExited},
		}},
		removeErr: map[string]error{"exited": errors.New("remove failed")},
	}
	docker := testDocker(t, api)
	result, err := docker.Cleanup(context.Background())
	if err == nil || result.Found != 2 || result.Removed != 1 || result.Failed != 1 {
		t.Fatalf("Cleanup() = %#v, %v", result, err)
	}
	labels := api.listOpts.Filters["label"]
	for _, label := range []string{managedLabel + "=true", componentLabel + "=" + componentValue, workerLabel + "=worker-1"} {
		if !labels[label] {
			t.Errorf("cleanup filter missing %q: %#v", label, labels)
		}
	}
	if !slices.Equal(api.killIDs, []string{"running"}) {
		t.Errorf("kill IDs = %#v", api.killIDs)
	}
	if !slices.Equal(api.removeIDs, []string{"running", "exited"}) {
		t.Errorf("remove IDs = %#v", api.removeIDs)
	}
	for _, options := range api.removeOpts {
		if !options.Force || !options.RemoveVolumes {
			t.Errorf("remove options = %#v", options)
		}
	}
}

func TestDockerCloseIsIdempotent(t *testing.T) {
	api := &fakeDockerAPI{}
	docker := testDocker(t, api)
	if err := docker.Close(); err != nil {
		t.Fatal(err)
	}
	if err := docker.Close(); err != nil {
		t.Fatal(err)
	}
	if !api.closed {
		t.Fatal("Docker API client was not closed")
	}
}

func testDocker(t *testing.T, api dockerAPI) *Docker {
	t.Helper()
	docker, err := newDockerWithClient(DockerOptions{
		Endpoint: "unix:///var/run/docker.sock", AllowUnrestrictedDaemon: true, WorkerID: "worker-1", User: "65532:65532",
		AllowedRoots: []AllowedRoot{{Path: t.TempDir(), Writable: true}},
		APITimeout:   100 * time.Millisecond, StopTimeout: 100 * time.Millisecond,
	}, api)
	if err != nil {
		t.Fatalf("newDockerWithClient() error = %v", err)
	}
	return docker
}
