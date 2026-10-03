// Runs the codec program, whose path is the first argument, and checks its output.
// SHOWCASE_CODECS lists the codecs of the build, from clue.cue.
#include <cstdio>
#include <string>

#ifdef _WIN32
#define popen _popen
#define pclose _pclose
#endif

static std::string run(const std::string& exe, const std::string& args) {
	std::string cmd = "\"" + exe + "\" " + args;
#ifdef _WIN32
	// cmd.exe strips the outer quotes of the whole line.
	cmd = "\"" + cmd + "\"";
#endif
	std::string out;
	FILE* p = popen(cmd.c_str(), "r");
	if (p == nullptr) {
		return out;
	}
	char buf[256];
	while (std::fgets(buf, sizeof buf, p) != nullptr) {
		out += buf;
	}
	pclose(p);
	// Windows text mode aside, lines end with \n.
	std::string clean;
	for (char c : out) {
		if (c != '\r') {
			clean += c;
		}
	}
	return clean;
}

static int expect(const char* what, const std::string& got, const std::string& want) {
	if (got != want) {
		std::printf("FAIL %s:\n got  %s\n want %s\n", what, got.c_str(), want.c_str());
		return 1;
	}
	return 0;
}

int main(int argc, char** argv) {
	if (argc != 2) {
		std::puts("usage: cli_test <codec>");
		return 2;
	}
	std::string exe = argv[1];

	std::string list;
	for (char c : std::string(SHOWCASE_CODECS)) {
		list += c == ',' ? '\n' : c;
	}
	int failed = 0;
	failed += expect("list", run(exe, "list"), list + "\n");
	failed += expect("crc", run(exe, "crc 123456789"), "cbf43926\n");
	failed += expect("base16", run(exe, "encode base16 hi"), "6869\n");
	failed += expect("rot13", run(exe, "decode rot13 Uryyb"), "Hello\n");
	return failed == 0 ? 0 : 1;
}
