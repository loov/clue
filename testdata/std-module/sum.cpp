#include "sum.h"

int sum(const std::vector<int>& values) {
    return std::accumulate(values.begin(), values.end(), 0);
}
