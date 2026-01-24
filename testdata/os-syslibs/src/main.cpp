#include <iostream>
#include <pthread.h>
#include <string>

// Platform detection
#if defined(__linux__)
#define PLATFORM "Linux"
#elif defined(__APPLE__) && defined(__MACH__)
#define PLATFORM "macOS"
#elif defined(_WIN32)
#define PLATFORM "Windows"
#else
#define PLATFORM "Unknown"
#endif

// Thread function - simple counter
void* thread_function(void* arg) {
    int* count = static_cast<int*>(arg);
    for (int i = 0; i < 5; ++i) {
        (*count)++;
    }
    return nullptr;
}

int main() {
    std::cout << "OS-Specific System Libraries Demo\n";
    std::cout << "==================================\n";
    std::cout << "Platform: " << PLATFORM << "\n";

    // Demonstrate pthread usage (available on Linux and macOS)
    pthread_t thread;
    int counter = 0;

    std::cout << "Starting thread...\n";
    int result = pthread_create(&thread, nullptr, thread_function, &counter);
    if (result != 0) {
        std::cerr << "Error: pthread_create failed with code " << result << "\n";
        return 1;
    }

    // Wait for thread to complete
    pthread_join(thread, nullptr);

    std::cout << "Thread completed. Counter value: " << counter << "\n";
    std::cout << "Success! pthread linking works correctly.\n";

    return 0;
}
