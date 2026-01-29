#ifndef STRINGUTILS_H
#define STRINGUTILS_H

#include <string>

namespace stringutils {
    // Format a calculation result using simplemath
    std::string format_sum(int a, int b);
    std::string format_product(int a, int b);
    std::string format_square(int n);

    // Simple string helpers
    std::string repeat(const std::string& s, int times);
}

#endif
