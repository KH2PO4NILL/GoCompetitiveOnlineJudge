package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func TestExecuteRequiresFinalAccountingAndChecksCPUBudget(t *testing.T) {
	for _, test := range []struct {
		name  string
		final bool
		cpu   time.Duration
		want  ExitReason
	}{
		{"within budget", true, 20 * time.Millisecond, ExitNormal},
		{"overrun between samples", true, 200 * time.Millisecond, ExitTLE},
		{"exact budget", true, 100 * time.Millisecond, ExitTLE},
		{"missing final accounting", false, 0, ExitSandboxError},
	} {
		t.Run(test.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			resultCh := make(chan container.WaitResponse, 1)
			api := &executeDockerAPI{
				attachResult:  client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{Conn: clientConn, Reader: bufio.NewReader(clientConn)}},
				waitResult:    client.ContainerWaitResult{Result: resultCh, Error: make(chan error)},
				inspectResult: client.ContainerInspectResult{Container: container.InspectResponse{State: &container.State{}}},
			}
			api.start = func() {
				go func() {
					if test.final {
						var final [24]byte
						binary.BigEndian.PutUint64(final[:8], uint64(test.cpu))
						_ = writeSupervisorFrame(serverConn, 3, final[:])
					}
					_ = serverConn.Close()
					resultCh <- container.WaitResponse{}
				}()
			}
			request := validExecuteRequest()
			request.Mounts = nil
			request.Limits.CPUTime = 100 * time.Millisecond
			result, err := testDocker(t, api).Execute(context.Background(), request)
			if result.Reason != test.want || (err != nil) != (test.want == ExitSandboxError) || result.CPUTime != test.cpu {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
}

func TestSupervisorProtocolFragmentationAndForgery(t *testing.T) {
	var stdout, stderr bytes.Buffer
	output := &supervisorOutput{stdout: &stdout, stderr: &stderr}
	fakeFinal := append([]byte{3, 0, 0, 0, 24}, make([]byte, 24)...)
	frame := append([]byte{1, 0, 0, 0, byte(len(fakeFinal))}, fakeFinal...)
	for _, value := range frame {
		if _, err := output.Write([]byte{value}); err != nil {
			t.Fatal(err)
		}
	}
	if output.finished || !bytes.Equal(stdout.Bytes(), fakeFinal) {
		t.Fatal("user output forged accounting")
	}
	for _, invalid := range [][]byte{{4, 0, 0, 0, 0}, {1, 0, 1, 0, 0, 0}, {3, 0, 0, 0, 1, 0}} {
		decoder := &supervisorOutput{stdout: &stdout, stderr: &stderr}
		if _, err := decoder.Write(invalid); err == nil {
			t.Fatalf("accepted invalid frame %v", invalid)
		}
	}
}
