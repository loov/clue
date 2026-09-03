# Raylib container cross-compilation example

This project builds the same Raylib 6.0 program for 64-bit Linux, Windows,
and macOS. Each target selects a Containerfile containing its compiler, SDK,
and a matching static Raylib build. Clue builds the image with Docker, Podman,
Apple container, or nerdctl and reuses the runtime's build cache.

Run the opt-in integration test from the repository root:

```sh
CLUE_TEST_RAYLIB_CONTAINER=1 go test -run TestRaylibExampleCrossCompilesEveryDesktopTarget .
```

The test invokes Clue once per target and verifies the ELF, PE, and Mach-O
output headers. The runtime uses emulation for the `linux/amd64` toolchain
containers when the host is ARM64.

The macOS image uses OSXCross and a macOS SDK. Review and accept Apple's Xcode
license terms before building or using that image.
