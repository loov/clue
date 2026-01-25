---
phase: quick
plan: 021
type: execute
wave: 1
depends_on: []
files_modified: [README.md]
autonomous: true

must_haves:
  truths:
    - "User understands what Clue is"
    - "User knows how to install Clue"
    - "User can create a minimal project config"
    - "User knows basic build commands"
  artifacts:
    - path: "README.md"
      provides: "Project documentation"
      contains: "clue build"
  key_links: []
---

<objective>
Create a minimal README.md that explains what Clue is and how to use it for basic C++ projects.

Purpose: Help new users quickly understand and start using the build system.
Output: README.md at repository root with installation, quick start, and basic command reference.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@main.go (CLI commands and flags)
@testdata/multi-target/clue.cue (simple config example)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create minimal README.md</name>
  <files>README.md</files>
  <action>
Create README.md at repository root with these sections:

1. **Title and description** (1-2 sentences)
   - "Clue - A C++ Build System"
   - Key value prop: CUE-based configuration, minimal config for common cases

2. **Installation** (brief)
   - `go install github.com/loov/clue@latest`
   - Or `go build -o clue .` for development

3. **Quick Start** (minimal working example)
   - Show a simple clue.cue for a single executable:
     ```cue
     name: "hello"
     version: "1.0.0"
     toolchain: {
         compiler: "clang"
         std:      "c++17"
     }
     targets: {
         hello: {
             name:    "hello"
             type:    "executable"
             sources: ["main.cpp"]
         }
     }
     ```
   - Commands: `clue validate`, `clue build`

4. **Commands** (brief list)
   - `clue validate` - Validate configuration
   - `clue build` - Build project (add `-variant release` note)
   - `clue clean` - Clean build artifacts
   - `clue run <target>` - Build and run executable
   - `clue deps fetch` - Fetch external dependencies

5. **Common flags** (brief)
   - `-variant debug|release` - Build variant
   - `-j N` - Parallel jobs
   - `-v` - Verbose output

Keep total length under 100 lines. No badges, no CI status, no contributing section - just the essentials.
  </action>
  <verify>
    - File exists: `test -f README.md`
    - Contains key sections: `grep -q "Installation" README.md && grep -q "Quick Start" README.md && grep -q "clue build" README.md`
  </verify>
  <done>README.md exists with installation instructions, quick start example, and command reference</done>
</task>

</tasks>

<verification>
- README.md exists at repository root
- Contains working example of clue.cue configuration
- Documents core commands (validate, build, clean, run)
- Is concise (under 100 lines)
</verification>

<success_criteria>
- README.md created at repository root
- New user can understand what Clue is and get started within 2 minutes
- All documented commands are accurate per main.go
</success_criteria>

<output>
After completion, create `.planning/quick/021-add-minimal-readme-that-explains-basic-u/021-SUMMARY.md`
</output>
