#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/prctl.h>
#include <sys/resource.h>
#include <sys/wait.h>
#include <unistd.h>

static void fail(const char *message) {
    perror(message);
    _exit(125);
}

static void write_all(const void *buffer, size_t length) {
    const unsigned char *position = buffer;
    while (length > 0) {
        ssize_t count = write(STDOUT_FILENO, position, length);
        if (count < 0 && errno == EINTR) continue;
        if (count <= 0) fail("supervisor output");
        position += count;
        length -= (size_t)count;
    }
}

static void encode(unsigned char *target, uint64_t value, size_t length) {
    for (size_t index = 0; index < length; index++) {
        target[length - index - 1] = (unsigned char)value;
        value >>= 8;
    }
}

static void frame(unsigned char kind, const void *data, size_t length) {
    unsigned char header[5] = {kind};
    encode(header + 1, length, 4);
    write_all(header, sizeof(header));
    write_all(data, length);
}

static uint64_t metric(const char *path, const char *key, int required) {
    FILE *file = fopen(path, "re");
    if (file == NULL) {
        if (required) fail("open cgroup v2 accounting");
        return 0;
    }
    char line[256], name[128];
    unsigned long long value = 0;
    int found = 0;
    while (fgets(line, sizeof(line), file) != NULL) {
        if (key == NULL) {
            found = sscanf(line, "%llu", &value) == 1;
        } else if (sscanf(line, "%127s %llu", name, &value) == 2 && strcmp(name, key) == 0) {
            found = 1;
        }
        if (found) break;
    }
    fclose(file);
    if (!found && required) fail("read cgroup v2 accounting");
    return found ? (uint64_t)value : 0;
}

int main(int argc, char **argv) {
    if (argc < 4 || getpid() != 1 || prctl(PR_SET_DUMPABLE, 0) != 0) fail("protected supervisor PID 1 required");
    char *end = NULL;
    uint64_t cpu_limit = strtoull(argv[1], &end, 10);
    if (*argv[1] == '\0' || *end != '\0' || cpu_limit == 0) fail("invalid CPU limit");
    uint64_t open_files = strtoull(argv[2], &end, 10);
    if (*argv[2] == '\0' || *end != '\0' || open_files < 3 || open_files > 65536) fail("invalid file descriptor limit");
    metric("/sys/fs/cgroup/cpu.stat", "usage_usec", 1);
    int output[2], diagnostic[2];
    if (pipe2(output, O_CLOEXEC) != 0 || pipe2(diagnostic, O_CLOEXEC) != 0) fail("create child pipes");
    pid_t child = fork();
    if (child < 0) fail("fork child");
    if (child == 0) {
        if (dup2(output[1], STDOUT_FILENO) < 0 || dup2(diagnostic[1], STDERR_FILENO) < 0) fail("redirect child output");
        close(output[0]);
        close(output[1]);
        close(diagnostic[0]);
        close(diagnostic[1]);
        struct rlimit descriptors = {.rlim_cur = open_files, .rlim_max = open_files};
        if (setrlimit(RLIMIT_NOFILE, &descriptors) != 0) fail("set child descriptor limit");
        execv(argv[3], argv + 3);
        fail("exec child");
    }
    close(output[1]);
    close(diagnostic[1]);
    struct pollfd streams[2] = {{.fd = output[0], .events = POLLIN}, {.fd = diagnostic[0], .events = POLLIN}};
    int main_status = 0, main_exited = 0, children = 1;
    uint64_t cpu = 0, peak_memory = 0;
    uint32_t flags = 0;
    while (children || streams[0].fd >= 0 || streams[1].fd >= 0) {
        int status = 0;
        pid_t reaped;
        while ((reaped = waitpid(-1, &status, WNOHANG)) > 0) {
            if (reaped == child) {
                if (!WIFEXITED(status) && !WIFSIGNALED(status)) {
                    kill(-1, SIGKILL);
                    continue;
                }
                main_status = status;
                main_exited = 1;
                kill(-1, SIGKILL);
            }
        }
        if (reaped < 0 && errno != ECHILD && errno != EINTR) fail("reap children");
        if (reaped < 0 && errno == ECHILD) children = 0;
        cpu = metric("/sys/fs/cgroup/cpu.stat", "usage_usec", 1) * 1000;
        uint64_t memory = metric("/sys/fs/cgroup/memory.peak", NULL, 0);
        if (memory == 0) memory = metric("/sys/fs/cgroup/memory.current", NULL, 0);
        if (memory > peak_memory) peak_memory = memory;
        if (cpu >= cpu_limit && !(flags & 1)) {
            flags |= 1;
            kill(-1, SIGKILL);
        }
        if (poll(streams, 2, children ? 5 : 0) < 0 && errno != EINTR) fail("poll child output");
        for (size_t index = 0; index < 2; index++) {
            if (streams[index].fd < 0 || streams[index].revents == 0) continue;
            unsigned char buffer[16384];
            ssize_t count = read(streams[index].fd, buffer, sizeof(buffer));
            if (count > 0) frame((unsigned char)(index + 1), buffer, (size_t)count);
            else if (count == 0) {
                close(streams[index].fd);
                streams[index].fd = -1;
            } else if (errno != EINTR) fail("read child output");
        }
    }
    if (!main_exited) fail("missing child status");
    cpu = metric("/sys/fs/cgroup/cpu.stat", "usage_usec", 1) * 1000;
    uint64_t final_peak = metric("/sys/fs/cgroup/memory.peak", NULL, 0);
    if (final_peak > peak_memory) peak_memory = final_peak;
    if (cpu >= cpu_limit) flags |= 1;
    if (metric("/sys/fs/cgroup/memory.events", "oom_kill", 1) > 0) flags |= 2;
    unsigned char result[24];
    encode(result, cpu, 8);
    encode(result + 8, peak_memory, 8);
    encode(result + 16, WIFEXITED(main_status) ? WEXITSTATUS(main_status) : 128 + WTERMSIG(main_status), 4);
    encode(result + 20, flags, 4);
    frame(3, result, sizeof(result));
    return 0;
}
