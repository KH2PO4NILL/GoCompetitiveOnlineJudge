#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdio.h>
#include <string.h>
#include <sys/wait.h>
#include <sys/resource.h>
#include <time.h>
#include <unistd.h>

int main(int argc, char **argv) {
    if (argc < 2) return 64;
    if (strcmp(argv[1], "minimal-fds") == 0) {
        struct rlimit descriptors;
        if (getrlimit(RLIMIT_NOFILE, &descriptors) != 0 || descriptors.rlim_cur != 3 || descriptors.rlim_max != 3) return 55;
        if (open("/dev/null", O_RDONLY) >= 0 || errno != EMFILE) return 56;
        puts("limited");
        return 0;
    }
    if (strcmp(argv[1], "tamper") == 0) {
        if (open("/proc/1/fd/1", O_WRONLY) >= 0) return 51;
        if (open("/proc/1/mem", O_RDWR) >= 0) return 52;
        if (open("/sys/fs/cgroup/cpu.stat", O_WRONLY) >= 0) return 53;
        kill(1, SIGKILL);
        kill(1, SIGSTOP);
        unsigned char forged[29] = {3, 0, 0, 0, 24};
        fwrite(forged, 1, sizeof(forged), stdout);
        return 0;
    }
    if (strcmp(argv[1], "cpu-descendants") == 0) {
        signal(SIGCHLD, SIG_IGN);
        pid_t child = fork();
        if (child < 0) return 54;
        if (child > 0) { usleep(250000); return 0; }
        argv[1] = "cpu";
    }
    if (strcmp(argv[1], "cpu") == 0) {
        struct timespec elapsed;
        do { clock_gettime(CLOCK_PROCESS_CPUTIME_ID, &elapsed); }
        while (elapsed.tv_sec == 0 && elapsed.tv_nsec < 200000000);
        printf("cpu_ns=%ld\n", elapsed.tv_sec * 1000000000L + elapsed.tv_nsec);
        return 0;
    }
    if (strcmp(argv[1], "fork") == 0) {
        pid_t child = fork();
        if (child < 0) { printf("fork_errno=%d\n", errno); return 42; }
        if (child == 0) return 0;
        waitpid(child, NULL, 0);
        puts("fork_ok");
        return 0;
    }
    if (strcmp(argv[1], "hold") == 0) {
        for (int index = 0; index < 24; index++) {
            pid_t child = fork();
            if (child < 0) return 41;
            if (child == 0) { sleep(4); return 0; }
        }
        FILE *ready = fopen("/sandbox/control/ready", "w");
        if (!ready) return 43;
        fputs("ready", ready);
        fclose(ready);
        while (wait(NULL) > 0) {}
        return 0;
    }
    return 65;
}
