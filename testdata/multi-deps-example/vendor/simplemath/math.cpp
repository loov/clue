#include "arithmetic.h"

namespace simplemath {
    int add(int a, int b) {
        return a + b;
    }

    int multiply(int a, int b) {
        return a * b;
    }

    int square(int n) {
        return multiply(n, n);
    }
}
