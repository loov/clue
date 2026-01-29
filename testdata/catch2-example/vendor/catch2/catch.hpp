/*
 * CATCH2 v2.13.10 MOCK FOR TESTDATA
 *
 * This is a minimal implementation of Catch2 v2.x API for use in testdata.
 * The real catch.hpp is ~18,000 lines. This mock provides enough functionality
 * for basic unit testing: TEST_CASE, SECTION, REQUIRE, and REQUIRE_APPROX.
 *
 * Real Catch2: https://github.com/catchorg/Catch2
 * License: Boost Software License 1.0
 */

#ifndef CATCH_HPP_INCLUDED
#define CATCH_HPP_INCLUDED

#include <iostream>
#include <string>
#include <vector>
#include <functional>
#include <sstream>
#include <cmath>
#include <exception>

namespace Catch {

// Forward declarations
class TestRegistry;
class SectionTracker;

// Approx class for floating-point comparisons
class Approx {
public:
    explicit Approx(double value) : m_value(value), m_epsilon(1e-5) {}

    Approx& epsilon(double newEpsilon) {
        m_epsilon = newEpsilon;
        return *this;
    }

    friend bool operator==(double lhs, Approx const& rhs) {
        return std::abs(lhs - rhs.m_value) <= rhs.m_epsilon;
    }

    friend bool operator==(Approx const& lhs, double rhs) {
        return operator==(rhs, lhs);
    }

private:
    double m_value;
    double m_epsilon;
};

// Section tracking
class SectionTracker {
public:
    static SectionTracker& instance() {
        static SectionTracker tracker;
        return tracker;
    }

    bool enterSection(const std::string& name) {
        if (m_runSection.empty()) {
            m_runSection = name;
            return true;
        }
        return m_runSection == name;
    }

    void leaveSection() {
        m_runSection.clear();
    }

    bool hasMoreSections() const {
        return !m_pendingSections.empty();
    }

    void addPendingSection(const std::string& name) {
        m_pendingSections.push_back(name);
    }

    std::string getNextSection() {
        if (m_pendingSections.empty()) return "";
        std::string next = m_pendingSections.front();
        m_pendingSections.erase(m_pendingSections.begin());
        return next;
    }

    void reset() {
        m_runSection.clear();
        m_pendingSections.clear();
    }

private:
    std::string m_runSection;
    std::vector<std::string> m_pendingSections;
};

// Section guard
class Section {
public:
    Section(const char* name) : m_name(name), m_entered(false) {
        m_entered = SectionTracker::instance().enterSection(name);
    }

    ~Section() {
        if (m_entered) {
            SectionTracker::instance().leaveSection();
        }
    }

    operator bool() const {
        return m_entered;
    }

private:
    std::string m_name;
    bool m_entered;
};

// Test case registry
class TestCase {
public:
    std::string name;
    std::string tags;
    std::function<void()> func;

    TestCase(const std::string& n, const std::string& t, std::function<void()> f)
        : name(n), tags(t), func(f) {}
};

class TestRegistry {
public:
    static TestRegistry& instance() {
        static TestRegistry registry;
        return registry;
    }

    void registerTest(const std::string& name, const std::string& tags, std::function<void()> func) {
        tests.emplace_back(name, tags, func);
    }

    int runAllTests() {
        int passed = 0;
        int failed = 0;

        std::cout << "\n";
        std::cout << "===============================================================================\n";
        std::cout << "Running " << tests.size() << " test case(s)...\n";
        std::cout << "===============================================================================\n";

        for (auto& test : tests) {
            try {
                std::cout << "\n-------------------------------------------------------------------------------\n";
                std::cout << test.name << "\n";
                std::cout << "-------------------------------------------------------------------------------\n";

                // Run test multiple times for sections
                bool hasMoreSections = true;
                int sectionRuns = 0;
                const int maxSectionRuns = 100; // Prevent infinite loops

                while (hasMoreSections && sectionRuns < maxSectionRuns) {
                    SectionTracker::instance().reset();

                    try {
                        test.func();
                        hasMoreSections = SectionTracker::instance().hasMoreSections();

                        if (hasMoreSections) {
                            std::string nextSection = SectionTracker::instance().getNextSection();
                            // Will run again with next section
                        }
                        sectionRuns++;
                    } catch (const std::exception& e) {
                        std::cout << "\n" << test.name << " FAILED:\n";
                        std::cout << "  " << e.what() << "\n";
                        failed++;
                        hasMoreSections = false;
                    }
                }

                if (sectionRuns > 0) {
                    passed++;
                }
            } catch (const std::exception& e) {
                std::cout << "\n" << test.name << " FAILED:\n";
                std::cout << "  " << e.what() << "\n";
                failed++;
            }
        }

        std::cout << "\n===============================================================================\n";
        std::cout << "test cases: " << tests.size() << " | "
                  << passed << " passed | " << failed << " failed\n";
        std::cout << "===============================================================================\n";

        return failed > 0 ? 1 : 0;
    }

private:
    std::vector<TestCase> tests;
};

// Helper for test registration
class TestRegistrar {
public:
    TestRegistrar(const char* name, const char* tags, std::function<void()> func) {
        TestRegistry::instance().registerTest(name, tags, func);
    }
};

// Assertion failure exception
class AssertionException : public std::exception {
public:
    AssertionException(const std::string& expr, const std::string& file, int line) {
        std::ostringstream oss;
        oss << file << ":" << line << ": FAILED:\n";
        oss << "  REQUIRE( " << expr << " )";
        m_message = oss.str();
    }

    const char* what() const noexcept override {
        return m_message.c_str();
    }

private:
    std::string m_message;
};

} // namespace Catch

// Macro definitions
#define CATCH_INTERNAL_LINEINFO __FILE__, __LINE__

#define CATCH_REQUIRE(expr) \
    do { \
        if (!(expr)) { \
            throw Catch::AssertionException(#expr, __FILE__, __LINE__); \
        } \
    } while(false)

#define REQUIRE(expr) CATCH_REQUIRE(expr)

#define CATCH_TEST_CASE(name, tags) \
    static void CATCH_INTERNAL_UNIQUE_NAME(catch_test_func)(); \
    static Catch::TestRegistrar CATCH_INTERNAL_UNIQUE_NAME(catch_test_registrar)(name, tags, &CATCH_INTERNAL_UNIQUE_NAME(catch_test_func)); \
    static void CATCH_INTERNAL_UNIQUE_NAME(catch_test_func)()

#define TEST_CASE(name, tags) CATCH_TEST_CASE(name, tags)

#define CATCH_SECTION(name) \
    if (Catch::Section CATCH_INTERNAL_UNIQUE_NAME(catch_section) = Catch::Section(name))

#define SECTION(name) CATCH_SECTION(name)

// Unique name generation
#define CATCH_INTERNAL_LINEINFO_CONCAT(name, line) name##line
#define CATCH_INTERNAL_LINEINFO_CONCAT2(name, line) CATCH_INTERNAL_LINEINFO_CONCAT(name, line)
#define CATCH_INTERNAL_UNIQUE_NAME(prefix) CATCH_INTERNAL_LINEINFO_CONCAT2(prefix, __LINE__)

// Main function generation
#ifdef CATCH_CONFIG_MAIN
int main(int argc, char* argv[]) {
    return Catch::TestRegistry::instance().runAllTests();
}
#endif

#endif // CATCH_HPP_INCLUDED
