#include <iostream>
#include <fstream>
#include <nlohmann/json.hpp>

using json = nlohmann::json;

int main() {
	// Open the JSON file
	std::ifstream file("sample.json");
	if (!file.is_open()) {
		std::cerr << "Error: Could not open sample.json" << std::endl;
		return 1;
	}

	// Parse JSON
	json data;
	try {
		data = json::parse(file);
	} catch (const json::parse_error& e) {
		std::cerr << "JSON parse error: " << e.what() << std::endl;
		return 1;
	}

	// Output parsed values for test verification
	std::cout << "Parsed name: " << data["name"].get<std::string>() << std::endl;
	std::cout << "Version: " << data["version"].get<int>() << std::endl;

	// Process items array
	if (data.contains("items") && data["items"].is_array()) {
		std::cout << "Items count: " << data["items"].size() << std::endl;

		for (const auto& item : data["items"]) {
			std::string key = item["key"].get<std::string>();
			int value = item["value"].get<int>();
			std::cout << "  Item: key=" << key << ", value=" << value << std::endl;
		}
	}

	return 0;
}
