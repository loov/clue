#include "utils.h"
#include "simplemath/math.h"
#include <sstream>

namespace stringutils {
    std::string format_sum(int a, int b) {
        int result = simplemath::add(a, b);
        std::ostringstream oss;
        oss << a << " + " << b << " = " << result;
        return oss.str();
    }

    std::string format_product(int a, int b) {
        int result = simplemath::multiply(a, b);
        std::ostringstream oss;
        oss << a << " * " << b << " = " << result;
        return oss.str();
    }

    std::string format_square(int n) {
        int result = simplemath::square(n);
        std::ostringstream oss;
        oss << n << "^2 = " << result;
        return oss.str();
    }

    std::string repeat(const std::string& s, int times) {
        std::string result;
        for (int i = 0; i < times; ++i) {
            result += s;
        }
        return result;
    }
}
