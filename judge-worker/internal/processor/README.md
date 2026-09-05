# Lease-aware Processor

`Processor.Process(ctx, JudgeTaskEvent)` has no MQ dependency. Fake gateways and manual
events use exactly the same path as the consumer. Only successful Acquire can access
source/spec/data; submission, attempt and Problem revision must match the response.
The Attempt ID is the submission execution revision (rejudge/recovery creates a new ID).

Acquire Finished/Stale/LeasedByOther and Report Applied/Duplicate/Stale return ACK.
Dependency failures return Retry; malformed events and unconfigured languages return
DeadLetter. RPC gateways use separate timeouts and `judge.worker` service credentials.
The current development endpoints are private plaintext gRPC; production must place them
on a trusted private network or a TLS-authenticated transport proxy.

Heartbeat starts before spec lookup/download, reports stage and completed cases, retries
transient failures with bounded exponential backoff and stops work before its confirmed
lease deadline. FailedPrecondition/NotFound cancels the Engine as an explicitly stale lease.
Processing cancellation joins the heartbeat goroutine; cache leases and Attempt workspaces
are always released. Worker/DB clocks must be synchronized.

Reports use stable `judge-<attempt-id>-v1` identifiers and identical bounded payloads for
all retries, while heartbeats continue. If a Report response is lost after apply, retry
converges to Duplicate; redelivery after process death converges to Finished. Sensitive
RPC errors, lease tokens, source and compiler output are never logged by Processor.
