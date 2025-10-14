# Code Style and Conventions

## Go Conventions
This project follows standard Go conventions and idiomatic Go patterns.

## Naming Conventions
- **Exported types/functions**: Start with uppercase letter (e.g., `Config`, `Load`)
- **Private types/functions**: Start with lowercase letter (e.g., `findConfig`, `setDefaults`)
- **Variables**: camelCase (e.g., `homeDir`, `serverName`)
- **Constants**: CamelCase or SCREAMING_SNAKE_CASE for package-level constants
- **Struct fields**: PascalCase for exported fields, camelCase for unexported

## Documentation
- All exported types, functions, and methods have doc comments
- Doc comments start with the name of the element being documented
- Example: `// Config represents the root configuration structure`
- Example: `// Load loads and parses the configuration file`

## Code Organization
- Package structure follows Go best practices:
  - `cmd/` for application entrypoints
  - `internal/` for private application code
  - Each package in a separate directory
- One package per directory
- Keep package names short and descriptive

## Error Handling
- Always check errors; don't ignore them
- Use `fmt.Errorf` with `%w` verb for error wrapping to preserve error chain
- Example: `return nil, fmt.Errorf("failed to read config file: %w", err)`
- Return errors as the last return value

## Struct Tags
- Use YAML struct tags for configuration structs
- Include `omitempty` for optional fields
- Example: `Args []string \`yaml:"args,omitempty"\``

## Function Structure
- Keep functions focused and single-purpose
- Prefer small, composable functions
- Use early returns to reduce nesting

## Comments
- Use `// TODO:` comments for planned implementations
- Keep comments concise and meaningful
- No redundant comments that just repeat the code

## Import Organization
- Standard library imports first
- Third-party imports after a blank line
- No need for explicit grouping comments

## Variable Declarations
- Use short variable declarations (`:=`) where appropriate
- Use `var` for zero values or when type needs to be explicit
- Example: `var config Config` before unmarshaling

## Method Receivers
- Use pointer receivers for methods that modify the receiver
- Use consistent receiver names (typically single letter or short abbreviation)
- Example: `func (c *Config) Validate() error`
