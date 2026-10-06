export module greet;

import std;

export std::string greet(std::string_view name) {
    return std::format("Hello, {}!", name);
}
