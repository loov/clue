#include <iostream>
#include "stringutils/utils.h"
#include "simplemath/math.h"

int main() {
    std::cout << "=== Multi-Deps Example ===" << std::endl;
    std::cout << std::endl;

    // Using stringutils (which uses simplemath internally)
    std::cout << "Using stringutils (chained dep):" << std::endl;
    std::cout << "  " << stringutils::format_sum(10, 5) << std::endl;
    std::cout << "  " << stringutils::format_product(7, 8) << std::endl;
    std::cout << "  " << stringutils::format_square(9) << std::endl;
    std::cout << std::endl;

    // Direct simplemath usage
    std::cout << "Using simplemath directly:" << std::endl;
    std::cout << "  add(100, 200) = " << simplemath::add(100, 200) << std::endl;
    std::cout << std::endl;

    // String utilities
    std::cout << "String repeat: " << stringutils::repeat("abc", 3) << std::endl;

    std::cout << std::endl;
    std::cout << "Multi-deps example complete!" << std::endl;

    return 0;
}
