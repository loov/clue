#define CATCH_CONFIG_MAIN
#include <catch2/catch.hpp>

#include <string>
#include <concepts>

// Basic math tests
TEST_CASE("Math operations", "[math]") {
	SECTION("addition") {
		REQUIRE(1 + 1 == 2);
		REQUIRE(2 + 2 == 4);
		REQUIRE(10 + 5 == 15);
	}

	SECTION("multiplication") {
		REQUIRE(2 * 3 == 6);
		REQUIRE(4 * 5 == 20);
		REQUIRE(7 * 8 == 56);
	}

	SECTION("division") {
		REQUIRE(10 / 2 == 5);
		REQUIRE(15 / 3 == 5);
		REQUIRE(100 / 4 == 25);
	}
}

// String tests
TEST_CASE("String operations", "[string]") {
	std::string hello = "Hello";
	std::string world = "World";

	SECTION("length") {
		REQUIRE(hello.length() == 5);
		REQUIRE(world.length() == 5);
	}

	SECTION("concatenation") {
		REQUIRE(hello + " " + world == "Hello World");
		REQUIRE(hello + world == "HelloWorld");
	}

	SECTION("comparison") {
		REQUIRE(hello == "Hello");
		REQUIRE(world != "hello");
	}

	SECTION("substring") {
		REQUIRE(hello.substr(0, 4) == "Hell");
		REQUIRE(world.substr(1, 3) == "orl");
	}
}

// C++20 concepts test (demonstrates modern C++)
template<typename T>
concept Numeric = std::integral<T> || std::floating_point<T>;

template<Numeric T>
T add(T a, T b) {
	return a + b;
}

template<Numeric T>
T multiply(T a, T b) {
	return a * b;
}

TEST_CASE("C++20 concepts", "[cpp20]") {
	SECTION("integer operations") {
		REQUIRE(add(1, 2) == 3);
		REQUIRE(add(10, 20) == 30);
		REQUIRE(multiply(3, 4) == 12);
	}

	SECTION("floating point operations") {
		REQUIRE(add(1.5, 2.5) == 4.0);
		REQUIRE(add(0.1, 0.2) == Catch::Approx(0.3));
		REQUIRE(multiply(2.5, 4.0) == 10.0);
	}
}

// Edge cases and boundary tests
TEST_CASE("Edge cases", "[edge]") {
	SECTION("zero values") {
		REQUIRE(0 + 0 == 0);
		REQUIRE(0 * 100 == 0);
	}

	SECTION("negative numbers") {
		REQUIRE(-1 + 1 == 0);
		REQUIRE(-5 * 2 == -10);
		REQUIRE(add(-3, -7) == -10);
	}

	SECTION("empty strings") {
		std::string empty = "";
		REQUIRE(empty.length() == 0);
		REQUIRE(empty + "test" == "test");
	}
}
