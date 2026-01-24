module;

#include <iostream>

export module hello;

export void say_hello() {
    std::cout << "Hello from module!" << std::endl;
}
