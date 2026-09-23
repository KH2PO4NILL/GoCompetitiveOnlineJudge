ARG BASE_IMAGE=golang:1.26.4-bookworm
FROM ${BASE_IMAGE} AS compiler
COPY --chmod=0555 go/build /opt/judge/build
COPY go/runner.go /opt/judge/runner.go
COPY supervisor.c /opt/judge/supervisor.c
RUN ["gcc", "-static", "-Os", "-Wall", "-Wextra", "-Werror", "-o", "/opt/judge/supervisor", "/opt/judge/supervisor.c"]
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
RUN ["/usr/local/go/bin/go", "build", "-trimpath", "-ldflags=-buildid=", "-o", "/opt/judge/run", "/opt/judge/runner.go"]
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]

FROM scratch AS runtime
COPY --from=compiler /opt/judge/run /opt/judge/run
COPY --from=compiler /opt/judge/supervisor /opt/judge/supervisor
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]
