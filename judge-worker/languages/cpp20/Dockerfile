ARG BASE_IMAGE=gcc:14.2.0-bookworm
FROM ${BASE_IMAGE} AS compiler
COPY --chmod=0555 build /opt/judge/build
COPY runner.c /opt/judge/runner.c
RUN ["/usr/local/bin/gcc", "-static", "-Os", "-o", "/opt/judge/run", "/opt/judge/runner.c"]
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]

FROM scratch AS runtime
COPY --from=compiler /opt/judge/run /opt/judge/run
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]
