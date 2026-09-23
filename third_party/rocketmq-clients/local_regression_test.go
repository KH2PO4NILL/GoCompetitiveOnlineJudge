package golang

import (
	"crypto/tls"
	"sync"
	"testing"
	"time"

	protocol "github.com/apache/rocketmq-clients/golang/v5/protocol/v2"
	"google.golang.org/protobuf/proto"
)

func TestLocalProducerRetrySnapshots(t *testing.T) {
	settings := &producerSettings{retryPolicy: &protocol.RetryPolicy{MaxAttempts: 3}}
	snapshot := settings.GetRetryPolicy()
	snapshot.MaxAttempts = 99
	if settings.GetRetryPolicy().MaxAttempts != 3 {
		t.Fatal("mutable retry settings escaped")
	}
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for range 1000 {
			err := settings.applySettingsCommand(&protocol.Settings{PubSub: &protocol.Settings_Publishing{Publishing: &protocol.Publishing{}}, BackoffPolicy: &protocol.RetryPolicy{MaxAttempts: 5}})
			if err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		for range 1000 {
			if _, err := proto.Marshal(settings.GetRetryPolicy()); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	workers.Wait()
}

func TestLocalManagerStopsSleepingMaintenance(t *testing.T) {
	manager := NewDefaultClientManager()
	manager.opts.RPC_CLIENT_IDLE_CHECK_INITIAL_DELAY = time.Hour
	manager.opts.HEART_BEAT_INITIAL_DELAY = time.Hour
	manager.opts.LOG_STATS_INITIAL_DELAY = time.Hour
	manager.opts.SYNC_SETTINGS_DELAY = time.Hour
	manager.startUp()
	done := make(chan struct{})
	go func() { manager.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("maintenance timers outlived close")
	}
	manager.Close()
}

func TestLocalMetricResetConcurrentWithMessageHooks(t *testing.T) {
	provider := &defaultClientMeterProvider{
		client:      &defaultClient{clientID: "metric-race-test"},
		clientMeter: NewDefaultClientMeter(nil, false, nil, "metric-race-test"),
	}
	interceptor := NewDefaultMessageMeterInterceptor(provider)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		for range 1000 {
			provider.Reset(&protocol.Metric{On: false})
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for range 1000 {
			if err := interceptor.doBefore(MessageHookPoints_SEND, nil); err != nil {
				t.Error(err)
				return
			}
			if err := interceptor.doAfter(MessageHookPoints_SEND, nil, time.Millisecond, MessageHookPointsStatus_OK); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	close(start)
	workers.Wait()
	if provider.isEnabled() {
		t.Fatal("disabled metrics became enabled")
	}
}

func TestLocalTransportSecurityDefaults(t *testing.T) {
	if defaultConnOptions.TLS.InsecureSkipVerify || defaultConnOptions.TLS.MinVersion < tls.VersionTLS12 {
		t.Fatal("unsafe default TLS verification")
	}
	if defaultConnOptions.MaxCallRecvMsgSize > 1<<20 || defaultConnOptions.MaxCallSendMsgSize >= defaultConnOptions.MaxCallRecvMsgSize {
		t.Fatal("unbounded or inconsistent frame limits")
	}
}
