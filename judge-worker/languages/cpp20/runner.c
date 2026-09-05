#include <unistd.h>

int main(int argc, char **argv) {
    (void)argc;
    argv[0] = "/sandbox/artifact/main";
    execv(argv[0], argv);
    return 127;
}
