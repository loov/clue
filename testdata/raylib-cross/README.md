# Raylib Docker cross-compilation example

This project builds the same Raylib 6.0 program for 64-bit Linux, Windows,
and macOS. Each target selects a Docker image containing its compiler, SDK,
and a matching static Raylib build.

Run the opt-in integration test from the repository root:

```sh
CLUE_TEST_RAYLIB_DOCKER=1 go test -run TestRaylibExampleCrossCompilesEveryDesktopTarget .
```

The test builds all three images for `linux/amd64`, invokes Clue once per
target, and verifies the ELF, PE, and Mach-O output headers. Docker uses
emulation when the host is ARM64.

The macOS image uses OSXCross and a macOS SDK. Review and accept Apple's Xcode
license terms before building or using that image.
