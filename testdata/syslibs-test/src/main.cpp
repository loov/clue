#include <cmath>
#include <cstdio>

int main() {
    double x = 2.0;
    double result = sqrt(x);
    printf("sqrt(%.1f) = %.6f\n", x, result);
    return (result > 1.4 && result < 1.5) ? 0 : 1;
}
