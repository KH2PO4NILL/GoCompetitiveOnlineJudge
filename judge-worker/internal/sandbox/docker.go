package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"golang.org/x/sys/unix"
)

const (
	managedLabel    = "gcoj.managed"
	componentLabel  = "gcoj.component"
	workerLabel     = "gcoj.worker_id"
	attemptLabel    = "gcoj.attempt_id"
	submissionLabel = "gcoj.submission_id"
	componentValue  = "judge-sandbox"
)

type DockerOptions struct {
	Endpoint                string
	AllowUnrestrictedDaemon bool
	WorkerID                string
	User                    string
	AllowedRoots            []AllowedRoot
	APITimeout              time.Duration
	StopTimeout             time.Duration
	NanoCPUs                int64
	Logger                  *slog.Logger
}

type dockerAPI interface {
	Ping(context.Context, client.PingOptions) (client.PingResult, error)
	ContainerCreate(context.Context, client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	ContainerAttach(context.Context, string, client.ContainerAttachOptions) (client.ContainerAttachResult, error)
	ContainerWait(context.Context, string, client.ContainerWaitOptions) client.ContainerWaitResult
	ContainerStart(context.Context, string, client.ContainerStartOptions) (client.ContainerStartResult, error)
	ContainerStats(context.Context, string, client.ContainerStatsOptions) (client.ContainerStatsResult, error)
	ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ContainerKill(context.Context, string, client.ContainerKillOptions) (client.ContainerKillResult, error)
	ContainerRemove(context.Context, string, client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error)
	Close() error
}

type Docker struct {
	client      dockerAPI
	workerID    string
	user        string
	roots       []allowedRoot
	apiTimeout  time.Duration
	stopTimeout time.Duration
	nanoCPUs    int64
	logger      *slog.Logger
	closeOnce   sync.Once
	closeErr    error
}

type resourceUsage struct {
	cpuNanos   atomic.Uint64
	peakMemory atomic.Uint64
}

type monitorEvent struct {
	reason ExitReason
	err    error
}

func NewDocker(options DockerOptions) (*Docker, error) {
	endpoint, err := validateDockerEndpoint(options.Endpoint, options.AllowUnrestrictedDaemon)
	if err != nil {
		return nil, err
	}
	options.Endpoint = endpoint
	apiClient, err := client.New(client.WithHost(endpoint), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create Docker API client: %w", err)
	}
	docker, err := newDockerWithClient(options, apiClient)
	if err != nil {
		_ = apiClient.Close()
		return nil, err
	}
	return docker, nil
}

func newDockerWithClient(options DockerOptions, apiClient dockerAPI) (*Docker, error) {
	if _, err := validateDockerEndpoint(options.Endpoint, options.AllowUnrestrictedDaemon); err != nil {
		return nil, err
	}
	if apiClient == nil {
		return nil, errors.New("Docker API client is required")
	}
	workerID := strings.TrimSpace(options.WorkerID)
	if workerID == "" || len(workerID) > 128 || strings.ContainsAny(workerID, "\r\n\x00") {
		return nil, errors.New("Docker sandbox requires a bounded worker ID")
	}
	user := strings.TrimSpace(options.User)
	if !validDockerUser(user) {
		return nil, errors.New("Docker sandbox requires a non-root numeric UID or UID:GID")
	}
	roots, err := resolveAllowedRoots(options.AllowedRoots)
	if err != nil {
		return nil, err
	}
	if options.APITimeout <= 0 {
		options.APITimeout = 10 * time.Second
	}
	if options.StopTimeout <= 0 {
		options.StopTimeout = DefaultStopTimeout
	}
	if options.APITimeout > MaxAPITimeout {
		return nil, errors.New("Docker sandbox API timeout exceeds hard limit")
	}
	if options.StopTimeout > MaxStopTimeout {
		return nil, errors.New("Docker sandbox stop timeout exceeds hard limit")
	}
	if options.NanoCPUs <= 0 {
		options.NanoCPUs = DefaultNanoCPUs
	}
	if options.NanoCPUs > int64(64)*DefaultNanoCPUs {
		return nil, errors.New("Docker sandbox NanoCPUs exceeds hard limit")
	}
	if options.Logger == nil {
		options.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Docker{
		client: apiClient, workerID: workerID, user: user, roots: roots,
		apiTimeout: options.APITimeout, stopTimeout: options.StopTimeout,
		nanoCPUs: options.NanoCPUs, logger: options.Logger,
	}, nil
}

func validateDockerEndpoint(endpoint string, allowUnrestricted bool) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", errors.New("Docker sandbox endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Opaque != "" {
		return "", errors.New("Docker sandbox endpoint must be a valid unix or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("Docker sandbox endpoint must not contain credentials, query parameters or fragments")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "unix":
		if parsed.Host != "" || !filepath.IsAbs(parsed.Path) || filepath.Clean(parsed.Path) == string(filepath.Separator) {
			return "", errors.New("Docker sandbox unix endpoint must contain an absolute socket path")
		}
		cleaned := filepath.Clean(parsed.Path)
		if !allowUnrestricted && (cleaned == "/var/run/docker.sock" || cleaned == "/run/docker.sock") {
			return "", errors.New("Docker sandbox must use a restricted API proxy; enable AllowUnrestrictedDaemon only on isolated development hosts")
		}
		return "unix://" + cleaned, nil
	case "https":
		if strings.TrimSpace(parsed.Hostname()) == "" || (parsed.Port() != "" && !validDockerPort(parsed.Port())) || (parsed.Path != "" && parsed.Path != "/") {
			return "", errors.New("Docker sandbox HTTPS endpoint must contain a valid host and optional port")
		}
		return endpoint, nil
	case "tcp", "http":
		return "", errors.New("Docker sandbox endpoint must not use an unauthenticated plaintext transport")
	default:
		return "", errors.New("Docker sandbox endpoint must use unix or https")
	}
}

func validDockerPort(port string) bool {
	value, err := strconv.Atoi(port)
	return err == nil && value >= 1 && value <= 65535
}

func validDockerUser(value string) bool {
	uidText, gidText, hasGID := strings.Cut(strings.TrimSpace(value), ":")
	uid, err := strconv.ParseUint(uidText, 10, 31)
	if err != nil || uid == 0 {
		return false
	}
	if !hasGID {
		return true
	}
	if gidText == "" || strings.Contains(gidText, ":") {
		return false
	}
	_, err = strconv.ParseUint(gidText, 10, 31)
	return err == nil
}

func (docker *Docker) Execute(parent context.Context, request ExecuteRequest) (result ExecuteResult, resultErr error) {
	if docker == nil || docker.client == nil {
		return ExecuteResult{Reason: ExitSandboxError}, errors.New("Docker sandbox is unavailable")
	}
	if parent == nil {
		parent = context.Background()
	}
	if request.WorkingDir == "" {
		request.WorkingDir = "/sandbox"
	}
	if err := request.Validate(); err != nil {
		return ExecuteResult{Reason: ExitSandboxError}, err
	}
	if err := parent.Err(); err != nil {
		return ExecuteResult{}, err
	}

	mounts, err := docker.mounts(request.Mounts)
	if err != nil {
		return ExecuteResult{Reason: ExitSandboxError}, fmt.Errorf("validate sandbox mounts: %w", err)
	}
	name, err := docker.containerName(request.AttemptID)
	if err != nil {
		return ExecuteResult{Reason: ExitSandboxError}, err
	}
	createOptions := docker.createOptions(request, mounts, name)
	operationCtx, operationCancel := context.WithTimeout(parent, docker.apiTimeout)
	created, err := docker.client.ContainerCreate(operationCtx, createOptions)
	operationCancel()
	if err != nil {
		// A timed-out create is ambiguous: the daemon may have created the
		// container even though the client never received its ID. The unique
		// name lets us reclaim that possible container immediately.
		if cleanupErr := docker.removeContainer(name); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		return operationFailure(parent, ExecuteResult{}, "create sandbox container", err)
	}
	result.ContainerID = created.ID
	defer func() {
		cleanupErr := docker.removeContainer(created.ID)
		if cleanupErr == nil {
			return
		}
		result.Reason = ExitSandboxError
		if result.Diagnostic == "" {
			result.Diagnostic = boundedDiagnostic(cleanupErr.Error())
		}
		if resultErr == nil {
			resultErr = cleanupErr
		} else {
			resultErr = errors.Join(resultErr, cleanupErr)
		}
	}()

	attachCtx, attachCancel := context.WithTimeout(parent, docker.apiTimeout)
	attached, err := docker.client.ContainerAttach(attachCtx, created.ID, client.ContainerAttachOptions{
		Stream: true, Stdin: request.Stdin != nil, Stdout: true, Stderr: true,
	})
	attachCancel()
	if err != nil {
		return operationFailure(parent, result, "attach sandbox container", err)
	}
	defer attached.Close()

	// Open the streaming statistics endpoint before starting the container so
	// Docker can begin collecting as early as its API permits. The one-shot
	// poller supplements the stream for CPU-limit enforcement.
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()
	resourceStream, err := docker.openResourceStream(parent, monitorCtx, created.ID)
	if err != nil {
		return operationFailure(parent, result, "open sandbox resource statistics", err)
	}
	usage := &resourceUsage{}
	streamDone := make(chan monitorEvent, 1)
	go docker.monitorResourceStream(monitorCtx, resourceStream.Body, request.Limits.CPUTime, usage, streamDone)
	streamCh := (<-chan monitorEvent)(streamDone)
	pollDone := make(chan monitorEvent, 1)
	// Poll before ContainerStart returns: very short processes can otherwise
	// finish during the start API call and never expose a running stats endpoint.
	go docker.pollResources(monitorCtx, created.ID, request.Limits.CPUTime, usage, pollDone)
	pollCh := (<-chan monitorEvent)(pollDone)
	monitorsStopped := false
	stopMonitors := func() bool {
		if monitorsStopped {
			return true
		}
		monitorCancel()
		monitorStopCtx, monitorStopCancel := context.WithTimeout(context.Background(), min(ResourceDrainTime, docker.stopTimeout))
		defer monitorStopCancel()
		streamStopped := drainMonitor(monitorStopCtx, streamDone)
		pollStopped := drainMonitor(monitorStopCtx, pollDone)
		monitorsStopped = streamStopped && pollStopped
		return monitorsStopped
	}
	defer stopMonitors()

	waitCtx, waitCancel := context.WithCancel(context.Background())
	defer waitCancel()
	waitResult := docker.client.ContainerWait(waitCtx, created.ID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNextExit,
	})
	waitResultCh := waitResult.Result
	waitErrorCh := waitResult.Error

	startedAt := time.Now()
	startCtx, startCancel := context.WithTimeout(parent, docker.apiTimeout)
	_, err = docker.client.ContainerStart(startCtx, created.ID, client.ContainerStartOptions{})
	startCancel()
	if err != nil {
		return operationFailure(parent, result, "start sandbox container", err)
	}

	budget := &outputBudget{limit: request.Limits.OutputBytes}
	stdout := &boundedBuffer{budget: budget}
	stderr := &boundedBuffer{budget: budget}
	outputDone := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(stdout, stderr, attached.Reader)
		outputDone <- err
	}()
	outputCh := (<-chan error)(outputDone)
	outputCompleted := false
	var stdinDone chan struct{}
	if request.Stdin != nil {
		stdinDone = make(chan struct{})
		go func() {
			defer close(stdinDone)
			_, _ = io.Copy(attached.Conn, request.Stdin)
			_ = attached.CloseWrite()
		}()
	} else {
		_ = attached.CloseWrite()
	}

	wallRemaining := request.Limits.WallTime - time.Since(startedAt)
	wallExpired := wallRemaining <= 0
	if wallExpired {
		wallRemaining = 0
	}
	wallTimer := time.NewTimer(wallRemaining)
	defer wallTimer.Stop()

	var waitResponse container.WaitResponse
	var forcedReason ExitReason
	var executionErr error
	waiting := true
	if wallExpired {
		forcedReason = ExitTLE
		docker.killContainer(created.ID)
		waitResponse, executionErr = docker.waitAfterKill(waitResult, executionErr)
		waiting = false
	}
	for waiting {
		select {
		case response, ok := <-waitResultCh:
			if !ok {
				waitResultCh = nil
				if waitErrorCh == nil {
					executionErr = errors.New("wait for sandbox container: Docker closed wait channels without a result")
					forcedReason = ExitSandboxError
					waiting = false
				}
				continue
			}
			waitResponse = response
			waiting = false
		case err, ok := <-waitErrorCh:
			if !ok {
				waitErrorCh = nil
				if waitResultCh == nil {
					executionErr = errors.New("wait for sandbox container: Docker closed wait channels without a result")
					forcedReason = ExitSandboxError
					waiting = false
				}
				continue
			}
			if err == nil {
				continue
			}
			executionErr = fmt.Errorf("wait for sandbox container: %w", err)
			forcedReason = ExitSandboxError
			waiting = false
		case event, ok := <-streamCh:
			if !ok {
				streamCh = nil
				continue
			}
			forcedReason, executionErr = event.reason, event.err
			docker.killContainer(created.ID)
			waitResponse, executionErr = docker.waitAfterKill(waitResult, executionErr)
			waiting = false
		case event, ok := <-pollCh:
			if !ok {
				pollCh = nil
				continue
			}
			forcedReason, executionErr = event.reason, event.err
			docker.killContainer(created.ID)
			waitResponse, executionErr = docker.waitAfterKill(waitResult, executionErr)
			waiting = false
		case outputErr := <-outputCh:
			outputCompleted = true
			outputCh = nil
			if errors.Is(outputErr, ErrOutputLimit) {
				forcedReason = ExitOLE
				// Once the bounded reader stops, a noisy process can fill Docker's
				// attach pipe and delay the kill request. Disconnect first so the
				// daemon can tear the process down promptly.
				attached.Close()
				docker.killContainer(created.ID)
				waitResponse, executionErr = docker.waitAfterKill(waitResult, executionErr)
				waiting = false
			} else if outputErr != nil {
				forcedReason = ExitSandboxError
				executionErr = fmt.Errorf("read sandbox output: %w", outputErr)
				docker.killContainer(created.ID)
				waitResponse, executionErr = docker.waitAfterKill(waitResult, executionErr)
				waiting = false
			}
		case <-wallTimer.C:
			forcedReason = ExitTLE
			docker.killContainer(created.ID)
			waitResponse, executionErr = docker.waitAfterKill(waitResult, executionErr)
			waiting = false
		case <-parent.Done():
			executionErr = parent.Err()
			docker.killContainer(created.ID)
			_, executionErr = docker.waitAfterKill(waitResult, executionErr)
			waiting = false
		}
	}
	result.WallTime = time.Since(startedAt)
	if !stopMonitors() && executionErr == nil && parent.Err() == nil {
		forcedReason = ExitSandboxError
		executionErr = errors.New("sandbox resource monitor did not close")
	}
	if request.Stdin != nil {
		// Close the caller-owned input after execution so a blocked Read is
		// interrupted. ExecuteRequest requires an io.ReadCloser for this reason.
		_ = request.Stdin.Close()
		if stdinDone != nil {
			select {
			case <-stdinDone:
			case <-time.After(docker.stopTimeout):
				if executionErr == nil && parent.Err() == nil {
					forcedReason = ExitSandboxError
					executionErr = errors.New("sandbox input stream did not close")
				}
			}
		}
	}
	if forcedReason != "" || parent.Err() != nil {
		// Docker may keep a hijacked attach connection open after a forced
		// termination. Closing it here unblocks output collection without
		// waiting for the daemon-side stream to time out.
		attached.Close()
	}

	// Closing the write side is not sufficient to unblock a damaged attach
	// stream. Give normal output a bounded drain window, then close it.
	if !outputCompleted {
		select {
		case outputErr := <-outputDone:
			if errors.Is(outputErr, ErrOutputLimit) && forcedReason == "" {
				forcedReason = ExitOLE
			} else if outputErr != nil && !errors.Is(outputErr, ErrOutputLimit) && executionErr == nil && forcedReason == "" && parent.Err() == nil {
				forcedReason = ExitSandboxError
				executionErr = fmt.Errorf("read sandbox output: %w", outputErr)
			}
		case <-time.After(docker.stopTimeout):
			attached.Close()
			if executionErr == nil {
				forcedReason = ExitSandboxError
				executionErr = errors.New("sandbox output stream did not close")
			}
		}
	}
	result.Stdout = stdout.Bytes()
	result.Stderr = stderr.Bytes()
	result.OutputBytes = budget.bytes()
	result.CPUTime = durationFromUint64(usage.cpuNanos.Load())
	result.PeakMemory = int64(min(usage.peakMemory.Load(), uint64(math.MaxInt64)))
	if forcedReason == "" && result.CPUTime >= request.Limits.CPUTime {
		forcedReason = ExitTLE
	}

	inspectCtx, inspectCancel := context.WithTimeout(context.Background(), docker.apiTimeout)
	inspected, inspectErr := docker.client.ContainerInspect(inspectCtx, created.ID, client.ContainerInspectOptions{})
	inspectCancel()
	if inspectErr != nil && executionErr == nil {
		forcedReason = ExitSandboxError
		executionErr = fmt.Errorf("inspect sandbox container: %w", inspectErr)
	}
	if inspected.Container.State != nil && inspected.Container.State.OOMKilled && result.PeakMemory < request.Limits.MemoryBytes {
		// Docker removes the cgroup immediately after a short OOM exit, so its
		// final stats sample may disappear before the client can read memory.peak.
		// An OOMKilled state proves the configured cgroup limit was reached; use
		// that limit as the conservative peak instead of returning a misleading
		// partial sample.
		result.PeakMemory = request.Limits.MemoryBytes
	}
	if waitResponse.Error != nil && waitResponse.Error.Message != "" && executionErr == nil {
		forcedReason = ExitSandboxError
		executionErr = fmt.Errorf("sandbox container wait error: %s", boundedDiagnostic(waitResponse.Error.Message))
	}

	if parent.Err() != nil && (errors.Is(executionErr, context.Canceled) || errors.Is(executionErr, context.DeadlineExceeded)) {
		result.Diagnostic = boundedDiagnostic(executionErr.Error())
		return result, executionErr
	}
	result = classifyResult(result, forcedReason, waitResponse.StatusCode, inspected.Container.State)
	if executionErr != nil {
		result.Reason = ExitSandboxError
		result.Diagnostic = boundedDiagnostic(executionErr.Error())
		return result, executionErr
	}
	return result, nil
}

func (docker *Docker) createOptions(request ExecuteRequest, mounts []mount.Mount, name string) client.ContainerCreateOptions {
	pids := request.Limits.PIDs
	initProcess := true
	memorySwappiness := int64(0)
	stopSeconds := max(1, int(docker.stopTimeout/time.Second))
	tmpfsBytes := min(request.Limits.MemoryBytes/4, int64(64<<20))
	if tmpfsBytes < 1<<20 {
		tmpfsBytes = 1 << 20
	}
	return client.ContainerCreateOptions{
		Name: name,
		Config: &container.Config{
			Image: request.Image, User: docker.user,
			Entrypoint: []string{request.Program}, Cmd: append([]string(nil), request.Args...),
			Env: append([]string(nil), request.Env...), WorkingDir: request.WorkingDir,
			AttachStdin: request.Stdin != nil, OpenStdin: request.Stdin != nil, StdinOnce: request.Stdin != nil,
			AttachStdout: true, AttachStderr: true, Tty: false, NetworkDisabled: true,
			Labels: map[string]string{
				managedLabel: "true", componentLabel: componentValue, workerLabel: docker.workerID,
				attemptLabel: fmt.Sprint(request.AttemptID), submissionLabel: fmt.Sprint(request.SubmissionID),
			},
			StopTimeout: &stopSeconds,
		},
		HostConfig: &container.HostConfig{
			Resources: container.Resources{
				Memory: request.Limits.MemoryBytes, MemorySwap: request.Limits.MemoryBytes,
				MemorySwappiness: &memorySwappiness, PidsLimit: &pids, NanoCPUs: docker.nanoCPUs,
				Ulimits: []*container.Ulimit{
					{Name: "nofile", Soft: int64(request.Limits.OpenFiles), Hard: int64(request.Limits.OpenFiles)},
					{Name: "nproc", Soft: request.Limits.PIDs, Hard: request.Limits.PIDs},
				},
			},
			NetworkMode: "none", RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
			AutoRemove: false, Privileged: false, ReadonlyRootfs: true,
			CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"},
			Mounts: mounts, Init: &initProcess, LogConfig: container.LogConfig{Type: "none"},
			Tmpfs: map[string]string{
				"/tmp": fmt.Sprintf("rw,noexec,nosuid,nodev,size=%d,mode=1777", tmpfsBytes),
			},
		},
	}
}

func (docker *Docker) mounts(items []Mount) ([]mount.Mount, error) {
	result := make([]mount.Mount, 0, len(items))
	for _, item := range items {
		source, err := secureMountSource(item.Source, item.ReadOnly, docker.roots)
		if err != nil {
			return nil, err
		}
		result = append(result, mount.Mount{
			Type: mount.TypeBind, Source: source, Target: item.Target, ReadOnly: item.ReadOnly,
			BindOptions: &mount.BindOptions{Propagation: mount.PropagationRPrivate},
		})
	}
	return result, nil
}

func (docker *Docker) openResourceStream(parent, streamCtx context.Context, containerID string) (client.ContainerStatsResult, error) {
	type response struct {
		stats client.ContainerStatsResult
		err   error
	}
	opened := make(chan response, 1)
	go func() {
		stats, err := docker.client.ContainerStats(streamCtx, containerID, client.ContainerStatsOptions{Stream: true})
		opened <- response{stats: stats, err: err}
	}()
	closeLateResponse := func() {
		go func() {
			result := <-opened
			if result.err == nil && result.stats.Body != nil {
				_ = result.stats.Body.Close()
			}
		}()
	}
	timer := time.NewTimer(docker.apiTimeout)
	defer timer.Stop()
	select {
	case result := <-opened:
		if result.err == nil && result.stats.Body == nil {
			return client.ContainerStatsResult{}, errors.New("Docker returned an empty resource statistics stream")
		}
		return result.stats, result.err
	case <-parent.Done():
		closeLateResponse()
		return client.ContainerStatsResult{}, parent.Err()
	case <-timer.C:
		closeLateResponse()
		return client.ContainerStatsResult{}, context.DeadlineExceeded
	}
}

func (docker *Docker) monitorResourceStream(ctx context.Context, body io.ReadCloser, cpuLimit time.Duration, usage *resourceUsage, done chan<- monitorEvent) {
	defer close(done)
	var closeOnce sync.Once
	closeBody := func() { closeOnce.Do(func() { _ = body.Close() }) }
	monitorDone := make(chan struct{})
	defer close(monitorDone)
	defer closeBody()
	go func() {
		select {
		case <-ctx.Done():
			closeBody()
		case <-monitorDone:
		}
	}()
	decoder := json.NewDecoder(body)
	for {
		var sample container.StatsResponse
		if err := decoder.Decode(&sample); err != nil {
			if ctx.Err() == nil && !errors.Is(err, io.EOF) {
				done <- monitorEvent{reason: ExitSandboxError, err: fmt.Errorf("read sandbox resource statistics: %w", err)}
			}
			return
		}
		if updateResourceUsage(&sample, cpuLimit, usage) {
			done <- monitorEvent{reason: ExitTLE}
			return
		}
	}
}

func (docker *Docker) pollResources(ctx context.Context, containerID string, cpuLimit time.Duration, usage *resourceUsage, done chan<- monitorEvent) {
	defer close(done)
	interval := resourcePollInterval(cpuLimit)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		requestTimeout := min(docker.apiTimeout, max(100*time.Millisecond, interval*4))
		requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		stats, err := docker.client.ContainerStats(requestCtx, containerID, client.ContainerStatsOptions{Stream: false})
		if err == nil {
			if stats.Body == nil {
				err = errors.New("Docker returned an empty resource statistics response")
			} else {
				var sample container.StatsResponse
				err = json.NewDecoder(stats.Body).Decode(&sample)
				_ = stats.Body.Close()
				if err == nil && updateResourceUsage(&sample, cpuLimit, usage) {
					cancel()
					done <- monitorEvent{reason: ExitTLE}
					return
				}
			}
		}
		if err == nil {
			cancel()
			timer.Reset(interval)
			continue
		}
		cancel()
		if ctx.Err() != nil {
			return
		}
		if cerrdefs.IsNotFound(err) || cerrdefs.IsConflict(err) {
			timer.Reset(interval)
			continue
		}
		done <- monitorEvent{reason: ExitSandboxError, err: fmt.Errorf("poll sandbox resource statistics: %w", err)}
		return
	}
}

func drainMonitor(ctx context.Context, done <-chan monitorEvent) bool {
	for {
		select {
		case _, ok := <-done:
			if !ok {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
}

func updateResourceUsage(sample *container.StatsResponse, cpuLimit time.Duration, usage *resourceUsage) bool {
	cpu := sample.CPUStats.CPUUsage.TotalUsage
	for current := usage.cpuNanos.Load(); cpu > current && !usage.cpuNanos.CompareAndSwap(current, cpu); current = usage.cpuNanos.Load() {
	}
	peak := sample.MemoryStats.MaxUsage
	if sample.MemoryStats.Usage > peak {
		peak = sample.MemoryStats.Usage
	}
	for current := usage.peakMemory.Load(); peak > current && !usage.peakMemory.CompareAndSwap(current, peak); current = usage.peakMemory.Load() {
	}
	return cpuLimit > 0 && cpu >= uint64(cpuLimit)
}

func resourcePollInterval(cpuLimit time.Duration) time.Duration {
	interval := cpuLimit / 20
	return min(MaxResourcePoll, max(MinResourcePoll, interval))
}

func (docker *Docker) waitAfterKill(wait client.ContainerWaitResult, existing error) (container.WaitResponse, error) {
	timer := time.NewTimer(docker.stopTimeout)
	defer timer.Stop()
	resultCh := wait.Result
	errorCh := wait.Error
	for {
		select {
		case response, ok := <-resultCh:
			if ok {
				return response, existing
			}
			resultCh = nil
		case err, ok := <-errorCh:
			if ok && err != nil {
				if existing != nil {
					return container.WaitResponse{}, errors.Join(existing, err)
				}
				return container.WaitResponse{}, err
			}
			if !ok {
				errorCh = nil
			}
		case <-timer.C:
			err := errors.New("sandbox container did not stop after kill")
			if existing != nil {
				err = errors.Join(existing, err)
			}
			return container.WaitResponse{}, err
		}
		if resultCh == nil && errorCh == nil {
			err := errors.New("Docker closed wait channels without a result")
			if existing != nil {
				err = errors.Join(existing, err)
			}
			return container.WaitResponse{}, err
		}
	}
}

func (docker *Docker) killContainer(containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), docker.apiTimeout)
	defer cancel()
	_, err := docker.client.ContainerKill(ctx, containerID, client.ContainerKillOptions{Signal: "KILL"})
	if err != nil && !cerrdefs.IsNotFound(err) && !cerrdefs.IsNotModified(err) {
		docker.logger.Error("kill sandbox container failed", "container_id", containerID, "error", err)
	}
}

func (docker *Docker) removeContainer(containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), docker.apiTimeout+docker.stopTimeout)
	defer cancel()
	_, err := docker.client.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("remove sandbox container %s: %w", containerID, err)
	}
	return nil
}

func (docker *Docker) Cleanup(ctx context.Context) (CleanupResult, error) {
	var result CleanupResult
	if docker == nil || docker.client == nil {
		return result, errors.New("Docker sandbox is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, docker.apiTimeout)
	ping, err := docker.client.Ping(pingCtx, client.PingOptions{NegotiateAPIVersion: true})
	pingCancel()
	if err != nil {
		return result, fmt.Errorf("connect to Docker daemon: %w", err)
	}
	if ping.OSType != "" && !strings.EqualFold(ping.OSType, "linux") {
		return result, fmt.Errorf("Docker sandbox requires a Linux daemon, got %q", ping.OSType)
	}
	filters := make(client.Filters).
		Add("label", managedLabel+"=true").
		Add("label", componentLabel+"="+componentValue).
		Add("label", workerLabel+"="+docker.workerID)
	listCtx, listCancel := context.WithTimeout(ctx, docker.apiTimeout)
	containers, err := docker.client.ContainerList(listCtx, client.ContainerListOptions{All: true, Filters: filters})
	listCancel()
	if err != nil {
		return result, fmt.Errorf("list orphaned sandbox containers: %w", err)
	}
	result.Found = len(containers.Items)
	var cleanupErrors []error
	for _, item := range containers.Items {
		if err := ctx.Err(); err != nil {
			cleanupErrors = append(cleanupErrors, err)
			break
		}
		if item.State == container.StateRunning || item.State == container.StatePaused || item.State == container.StateRestarting {
			docker.killContainer(item.ID)
		}
		if err := docker.removeContainer(item.ID); err != nil {
			result.Failed++
			cleanupErrors = append(cleanupErrors, err)
			continue
		}
		result.Removed++
	}
	if len(cleanupErrors) > 0 {
		return result, fmt.Errorf("clean orphaned sandbox containers: %w", errors.Join(cleanupErrors...))
	}
	return result, nil
}

func (docker *Docker) Close() error {
	if docker == nil || docker.client == nil {
		return nil
	}
	docker.closeOnce.Do(func() { docker.closeErr = docker.client.Close() })
	return docker.closeErr
}

func (docker *Docker) containerName(attemptID int64) (string, error) {
	var randomBytes [6]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", fmt.Errorf("generate sandbox container name: %w", err)
	}
	worker := sanitizeName(docker.workerID)
	return fmt.Sprintf("gcoj-%s-attempt-%d-%s", worker, attemptID, hex.EncodeToString(randomBytes[:])), nil
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('-')
		}
	}
	result := strings.Trim(builder.String(), "-_.")
	if result == "" {
		return "worker"
	}
	if len(result) > 48 {
		result = result[:48]
	}
	return result
}

func classifyResult(result ExecuteResult, forced ExitReason, statusCode int64, state *container.State) ExecuteResult {
	exitCode := int(statusCode)
	result.ExitCode = &exitCode
	if statusCode > 128 {
		result.Signal = unix.SignalName(syscall.Signal(statusCode - 128))
	}
	if forced != "" {
		result.Reason = forced
		return result
	}
	if state != nil && state.OOMKilled {
		result.Reason = ExitMLE
		return result
	}
	if result.Signal != "" {
		result.Reason = ExitSignal
		return result
	}
	result.Reason = ExitNormal
	return result
}

func operationFailure(parent context.Context, result ExecuteResult, operation string, err error) (ExecuteResult, error) {
	wrapped := fmt.Errorf("%s: %w", operation, err)
	if parent != nil && parent.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		result.Diagnostic = boundedDiagnostic(wrapped.Error())
		return result, wrapped
	}
	return sandboxFailure(result, wrapped)
}

func sandboxFailure(result ExecuteResult, err error) (ExecuteResult, error) {
	result.Reason = ExitSandboxError
	result.Diagnostic = boundedDiagnostic(err.Error())
	return result, err
}

func boundedDiagnostic(value string) string {
	const maxDiagnosticBytes = 4096
	if len(value) <= maxDiagnosticBytes {
		return value
	}
	return value[:maxDiagnosticBytes]
}

func durationFromUint64(value uint64) time.Duration {
	if value > uint64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(value)
}
