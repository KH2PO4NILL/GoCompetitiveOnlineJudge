#include <iostream>
#include <string>
#include <sys/mman.h>

int main() {
    std::string mode;
    std::cin >> mode;
    if (mode == "stamp") std::cout << __DATE__ << __TIME__ << __TIMESTAMP__;
    if (mode == "re") return 7;
    if (mode == "tle") for (;;) {}
    if (mode == "mle") for (;;) {
        auto memory = static_cast<volatile char *>(mmap(nullptr, 1048576, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0));
        if (memory == MAP_FAILED) return 8;
        for (int offset = 0; offset < 1048576; offset += 4096) memory[offset] = 1;
    }
    if (mode == "ole") for (;;) std::cout << std::string(4096, 'x');
    std::cout << (mode == "wa" ? 41 : 42) << '\n';
}
