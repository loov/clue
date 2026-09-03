package plan

// TranslateStdForMSVC translates C/C++ standard names to MSVC format.
// MSVC uses /std:c++17, /std:c++20, /std:c++latest etc.
func TranslateStdForMSVC(std string) string {
	// Handle C++ standards with prefix
	switch std {
	case "c++11", "gnu++11":
		return "c++14" // MSVC minimum is C++14, approximate C++11 with C++14
	case "c++14", "gnu++14":
		return "c++14"
	case "c++17", "gnu++17":
		return "c++17"
	case "c++20", "gnu++20":
		return "c++20"
	case "c++23", "gnu++23":
		return "c++latest" // MSVC uses latest for experimental C++23 features
	// Handle C standards with prefix
	case "c99", "gnu99", "c9x":
		return "c11" // Approximate C99 with C11 (MSVC has limited C99)
	case "c11", "gnu11", "c1x":
		return "c11"
	case "c17", "gnu17", "c18":
		return "c17"
	case "c2x", "gnu2x", "c23":
		return "clatest" // C23 via latest
	default:
		// Pass through for known MSVC values or fallback
		return std
	}
}
