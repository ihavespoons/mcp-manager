# Working with Claude Code and Serena MCP

This project is configured to work with [Claude Code](https://docs.anthropic.com/claude/docs/claude-code) and the [Serena MCP](https://github.com/lastmile-ai/serena) server for enhanced development assistance.

## What is Serena?

Serena is a Model Context Protocol (MCP) server that provides semantic code understanding tools. It enables Claude Code to:

- Navigate codebases using symbolic information (classes, functions, methods)
- Understand code structure without reading entire files
- Perform precise code edits using symbol-based operations
- Search for patterns and references efficiently
- Maintain project-specific memories for better context

## Onboarding Complete

Serena has been onboarded to this project with the following memory files created:

### Memory Files

1. **project_overview.md**
   - Project purpose and goals
   - Tech stack (Go 1.25.2, Cobra, YAML, Docker)
   - Project structure and architecture
   - Key components and development status

2. **code_style_and_conventions.md**
   - Go naming conventions
   - Documentation standards
   - Error handling patterns
   - Code organization principles

3. **suggested_commands.md**
   - Build commands (`make build`, `make install`, `make run`)
   - Testing commands (`make test`, `make coverage`)
   - Code quality commands (`make fmt`, `make vet`, `make lint`)
   - Dependency management (`make tidy`, `make deps`)
   - Application commands

4. **task_completion_checklist.md**
   - Steps to follow after making code changes
   - Quality checks (format, vet, lint)
   - Testing and build verification
   - Module management workflow

## Benefits for Development

With Serena MCP, Claude Code can:

- **Efficient Code Navigation**: Find symbols, functions, and types without reading entire files
- **Context-Aware Assistance**: Use project memories to understand conventions and patterns
- **Precise Edits**: Make symbol-level edits (replace methods, insert code, etc.)
- **Smart Search**: Find references and dependencies across the codebase
- **Project Continuity**: Remember project-specific information across sessions

## Working with Claude Code

When working on this project with Claude Code:

1. **Ask for specific help**: Claude Code has access to the entire project structure through Serena
2. **Reference the checklist**: Mention "follow the task completion checklist" after changes
3. **Leverage memories**: Claude Code knows the project structure, conventions, and commands
4. **Symbol-based requests**: You can ask Claude to "modify the Config.Validate method" and it will find it precisely

## Example Requests

- "Add a new command to the CLI for restarting servers"
- "Implement the Docker container management functions"
- "Add validation logic to the Config.Validate method"
- "Follow the task completion checklist" (runs fmt, vet, test, build)
- "Find all references to the Server struct"
- "Register mcp-manager with Claude Code"
- "Update the documentation to reflect recent changes"

## Updating Memories

If the project structure, conventions, or commands change significantly, you can ask Claude Code to update the Serena memories:

```
"Update the Serena memory files with the new [information]"
```

### Important: Memory Updates After Implementation

**IMPORTANT FOR CLAUDE CODE**: After completing any implementation work (new features, bug fixes, architecture changes), you MUST update the relevant Serena memory files to reflect the changes. This ensures project continuity across sessions.

**When to update memories:**
- After implementing new features
- After making significant changes to architecture or configuration
- After completing bug fixes that change behavior
- After adding or removing major components

**Which memories to update:**
- `implementation_status.md` - Always update with new features, changes, and current status
- `project_overview.md` - Update if architecture or major components change
- `mcp_architecture_learnings.md` - Update if learning new patterns or approaches
- `current_configuration.md` - Update if configuration format or options change

**How to update:**
1. Read the current memory file using `mcp__serena__read_memory`
2. Update it with the new information using `mcp__serena__write_memory`
3. Include dates, file references, and impact of changes
4. Keep the format consistent with existing entries

**Example workflow:**
```
User: "Add support for HTTP path configuration"
Claude: [Implements the feature]
Claude: [Updates implementation_status.md with the new feature, files changed, and impact]
```

This practice ensures that future sessions have complete context about what has been implemented and why.

## Serena Tools Reference

For Claude Code: See **[SERENA_TOOLS_GUIDE.md](./SERENA_TOOLS_GUIDE.md)** for complete documentation of all Serena tools, including:
- Detailed tool parameters and usage
- Decision trees for choosing the right tool
- Common workflows and patterns
- Best practices for token-efficient code navigation

### Quick Tool Reference

**Understanding code without reading entire files:**
- `get_symbols_overview` - See file structure before reading
- `find_symbol` - Find and read specific functions/types/methods
- `find_referencing_symbols` - Find where symbols are used

**Precise code editing:**
- `replace_symbol_body` - Replace entire function/method/type
- `insert_after_symbol` / `insert_before_symbol` - Add new code
- `rename_symbol` - Rename across entire codebase

**Project context:**
- `read_memory` / `write_memory` - Access project knowledge
- Meta-cognitive tools - Reflect on progress and completeness

## Resources

- [Claude Code Documentation](https://docs.anthropic.com/claude/docs/claude-code)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [Serena MCP](https://github.com/lastmile-ai/serena)
- [Serena Tools Guide (this project)](./SERENA_TOOLS_GUIDE.md)
