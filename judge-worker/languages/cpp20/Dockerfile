ARG BASE_IMAGE=gcc:14.2.0-bookworm
FROM ${BASE_IMAGE} AS compiler
COPY --chmod=0555 cpp20/build /opt/judge/build
COPY cpp20/runner.c /opt/judge/runner.c
COPY supervisor.c /opt/judge/supervisor.c
RUN ["/usr/local/bin/gcc", "-static", "-Os", "-Wall", "-Wextra", "-Werror", "-o", "/opt/judge/supervisor", "/opt/judge/supervisor.c"]
RUN ["/usr/local/bin/gcc", "-static", "-Os", "-o", "/opt/judge/run", "/opt/judge/runner.c"]
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]

FROM scratch AS runtime
COPY --from=compiler /opt/judge/run /opt/judge/run
COPY --from=compiler /opt/judge/supervisor /opt/judge/supervisor
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]
