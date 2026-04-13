## ADDED Requirements

### Requirement: Check dependency usage via go mod tools
The system SHALL use `go mod why -m <package>` to determine if a dependency is actually used in the module. It SHALL use `go mod graph` to show the dependency's import chain.

#### Scenario: Direct dependency in use
- **WHEN** `go mod why -m github.com/foo/bar` returns an import chain
- **THEN** the system SHALL report the package as used and include the import chain

#### Scenario: Unused dependency
- **WHEN** `go mod why -m github.com/foo/bar` returns "(main module does not need package ...)"
- **THEN** the system SHALL report the package as not directly imported

#### Scenario: go mod tools not available
- **WHEN** the `go` binary is not available or the working directory has no `go.mod`
- **THEN** the system SHALL skip `go mod` analysis gracefully and proceed with source scanning only

#### Scenario: go mod why timeout
- **WHEN** `go mod why` does not complete within 60 seconds
- **THEN** the system SHALL abort the command and report a timeout (not a failure)

### Requirement: Scan Go source files for import usage
The system SHALL scan `.go` source files (excluding test files, vendor directory, and hack scripts) for imports of a given package. For each file importing the package, the system SHALL extract a snippet of 5 lines of context around each usage.

#### Scenario: Package imported in source file
- **WHEN** a `.go` file contains `"github.com/foo/bar"` in its import block
- **THEN** the system SHALL report the file path and extract usage snippets

#### Scenario: Package not imported anywhere
- **WHEN** no `.go` files import the package
- **THEN** the system SHALL report no direct imports found

#### Scenario: Exclude test files
- **WHEN** a `_test.go` file imports the package
- **THEN** the system SHALL NOT include it in the import results

#### Scenario: Exclude vendor directory
- **WHEN** a file under `vendor/` imports the package
- **THEN** the system SHALL NOT include it in the import results

### Requirement: Detect test file coverage
The system SHALL check whether each file that imports a dependency has a corresponding test file.

#### Scenario: Corresponding test file exists
- **WHEN** `cmd/main.go` imports the package and `cmd/main_test.go` exists
- **THEN** the system SHALL report `hasTest: true` for that file

#### Scenario: No corresponding test file
- **WHEN** `internal/handler.go` imports the package and no `*_test.go` file exists in the same directory
- **THEN** the system SHALL report `hasTest: false` for that file
