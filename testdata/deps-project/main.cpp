#include <iostream>
#include "libmath/arithmetic.h"

int main() {
    std::cout << "3 + 4 = " << libmath::add(3, 4) << std::endl;
    std::cout << "3 * 4 = " << libmath::multiply(3, 4) << std::endl;
    return 0;
}
