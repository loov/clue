#include <iostream>
#include <cmath>  // Uses system math library
#include "math.h"

int main() {
    int result = add(2, 3);
    int product = multiply(4, 5);
    double sqroot = sqrt(16.0);  // From system -lm

    std::cout << "add(2,3) = " << result << std::endl;
    std::cout << "multiply(4,5) = " << product << std::endl;
    std::cout << "sqrt(16) = " << sqroot << std::endl;

    // Return 0 if all correct, nonzero otherwise
    if (result == 5 && product == 20 && sqroot == 4.0) {
        return 0;
    }
    return 1;
}
