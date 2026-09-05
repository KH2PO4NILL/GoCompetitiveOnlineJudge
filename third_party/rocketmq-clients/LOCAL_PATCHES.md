# Local compatibility patch

Source: Apache rocketmq-clients Go v5.1.4, copied from its published Go module.
The original Apache license and source headers are retained. Examples and upstream
tests are omitted; wire protobuf and public APIs are unchanged.

Real broker acceptance under Go 1.26.4 / protobuf 1.36.12 exposed concurrent writes
to producer retry settings while the telemetry command was being serialized,
including a protobuf size-mismatch error. `producer_options.go` locks updates and
returns defensive protobuf snapshots rather than the mutable shared pointer.

`client_manager.go` now cancels and joins its maintenance timers and closes RPC
connections; `client.go` invokes that cleanup, joins telemetry sessions, makes the
recovery wait cancellable and synchronizes startup-error publication. Without
these fixes SDK Close left background managers and connections running.

The root go.mod uses a local replace so builds/CI never depend on an uncommitted
module-cache modification. Remove this fork once an upstream release includes
equivalent fixes and passes real Broker race and shutdown acceptance.
