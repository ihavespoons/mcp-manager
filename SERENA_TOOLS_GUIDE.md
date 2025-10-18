# Serena MCP Tools - Complete Reference Guide for Claude Code

This guide provides comprehensive instructions for using Serena's semantic code tools effectively. These tools enable token-efficient code navigation, precise editing, and project memory management.

---

## Core Philosophy

**READ LESS, UNDERSTAND MORE**: Serena tools enable symbol-level navigation, so you should:
- ❌ AVOID reading entire files unless absolutely necessary
- ✅ USE symbolic tools to read only what you need
- ✅ START with overview tools, then drill down to specific symbols
- ✅ READ bodies only when you need to edit or deeply understand them

---

## Tool Categories

### 1. File System Navigation

#### `mcp__serena__list_dir`
**Purpose**: List files and directories in a path (optionally recursive)

**Parameters**:
- `relative_path` (string, required): Path relative to repo root (use "." for root)
- `recursive` (boolean, required): Whether to scan subdirectories
- `skip_ignored_files` (boolean, default: false): Skip gitignored files
- `max_answer_chars` (int, default: -1): Limit output length

**When to use**:
- Understanding project structure
- Finding which directories contain relevant files
- Getting a directory overview before deeper exploration

**Example**:
```json
{
  "relative_path": "cmd",
  "recursive": true,
  "skip_ignored_files": true
}
```

---

#### `mcp__serena__find_file`
**Purpose**: Find files matching a pattern (wildcards * and ?)

**Parameters**:
- `file_mask` (string, required): Filename or pattern (e.g., "*.go", "config.yaml")
- `relative_path` (string, required): Directory to search in

**When to use**:
- Looking for specific files by name/pattern
- Finding all files of a certain type
- Locating configuration files

**Example**:
```json
{
  "file_mask": "*_test.go",
  "relative_path": "."
}
```

---

### 2. Code Understanding Tools

#### `mcp__serena__get_symbols_overview`
**Purpose**: Get high-level overview of top-level symbols in a file WITHOUT reading bodies

**Parameters**:
- `relative_path` (string, required): Path to the file
- `max_answer_chars` (int, default: -1): Limit output length

**When to use**:
- ✅ **FIRST STEP when exploring a new file**
- Understanding file structure before detailed reads
- Seeing available functions, types, methods at a glance
- Deciding which symbols to read in detail

**Returns**: List of top-level symbols with names, kinds, and locations (NO bodies)

**Best Practice**: ALWAYS use this before reading full files or searching for symbols within a file

**Example**:
```json
{
  "relative_path": "internal/config/config.go"
}
```

---

#### `mcp__serena__find_symbol`
**Purpose**: Find symbols by name path pattern and optionally read their bodies/children

**Parameters**:
- `name_path` (string, required): Symbol name path pattern (see below)
- `relative_path` (string, optional): Restrict to file/directory
- `include_body` (boolean, default: false): Include symbol source code
- `depth` (int, default: 0): Include child symbols (1 for methods, 2 for nested)
- `substring_matching` (boolean, default: false): Match last segment as substring
- `include_kinds` (array[int], optional): LSP symbol kinds to include
- `exclude_kinds` (array[int], optional): LSP symbol kinds to exclude
- `max_answer_chars` (int, default: -1): Limit output length

**Name Path Matching Logic**:
- `method` - Matches any symbol named "method" anywhere (method, class/method, etc.)
- `class/method` - Matches "method" inside "class" (and nested_class/class/method)
- `/class/method` - Absolute path: ONLY top-level class with method
- `/class` - Only top-level symbols named "class"

**LSP Symbol Kinds** (for include_kinds/exclude_kinds):
- 1=file, 2=module, 3=namespace, 4=package, 5=class, 6=method, 7=property, 8=field
- 9=constructor, 10=enum, 11=interface, 12=function, 13=variable, 14=constant
- 15=string, 16=number, 17=boolean, 18=array, 19=object, 20=key, 21=null
- 22=enum member, 23=struct, 24=event, 25=operator, 26=type parameter

**When to use**:
- Finding specific functions, types, methods by name
- Reading symbol signatures without bodies (include_body=false)
- Understanding a type's methods (depth=1, include_body=false)
- Reading specific symbol implementation (include_body=true)

**Best Practices**:
1. If you know the exact name and file → use absolute name path with relative_path
2. If you need to see method signatures → use depth=1, include_body=false
3. Only set include_body=true when you actually need to read/edit the code
4. Use substring_matching=true when searching by partial names

**Examples**:
```json
// Find all structs named Config (no bodies)
{
  "name_path": "Config",
  "include_kinds": [23],
  "include_body": false
}

// Find and read the Validate method of Config struct in specific file
{
  "name_path": "Config/Validate",
  "relative_path": "internal/config/config.go",
  "include_body": true
}

// Get all methods of Server struct (signatures only)
{
  "name_path": "/Server",
  "relative_path": "internal/gateway/gateway.go",
  "depth": 1,
  "include_body": false
}
```

---

#### `mcp__serena__find_referencing_symbols`
**Purpose**: Find all places where a symbol is referenced/used

**Parameters**:
- `name_path` (string, required): Symbol name path (same logic as find_symbol)
- `relative_path` (string, required): File containing the symbol (MUST be a file, not directory)
- `include_kinds` (array[int], optional): Filter referencing symbol kinds
- `exclude_kinds` (array[int], optional): Exclude referencing symbol kinds
- `max_answer_chars` (int, default: -1): Limit output length

**When to use**:
- Understanding how a symbol is used across the codebase
- Finding all callers of a function
- Impact analysis before changing a symbol
- Ensuring backward compatibility when editing

**Returns**: List of symbols that reference the target, with code snippets

**Example**:
```json
{
  "name_path": "Server/Start",
  "relative_path": "internal/gateway/gateway.go"
}
```

---

#### `mcp__serena__search_for_pattern`
**Purpose**: Flexible regex pattern search across files (for non-symbolic searches)

**Parameters**:
- `substring_pattern` (string, required): Regular expression (DOTALL mode)
- `relative_path` (string, optional): Restrict to file/directory
- `restrict_search_to_code_files` (boolean, default: false): Only search analyzable code
- `paths_include_glob` (string, optional): Include pattern (e.g., "*.go", "**/*.yaml")
- `paths_exclude_glob` (string, optional): Exclude pattern (takes precedence)
- `context_lines_before` (int, default: 0): Lines of context before match
- `context_lines_after` (int, default: 0): Lines of context after match
- `max_answer_chars` (int, default: -1): Limit output length

**Pattern Notes**:
- DOTALL mode: `.` matches newlines
- Use non-greedy quantifiers (`.*?`) to avoid matching too much
- Don't use `.*` at start/end (wasteful), only in middle for complex patterns

**When to use**:
- Searching for string literals, comments, or non-code content
- Finding patterns that aren't symbols (error messages, config keys)
- Searching in non-code files (YAML, Markdown, JSON)
- Finding usage patterns that symbolic search misses

**When NOT to use**:
- ❌ Searching for function/type/method names → use find_symbol instead
- ❌ Finding code references → use find_referencing_symbols instead

**Examples**:
```json
// Find TODO comments
{
  "substring_pattern": "TODO:.*",
  "restrict_search_to_code_files": true
}

// Find error messages containing "failed to"
{
  "substring_pattern": "fmt\\.Errorf\\(\".*failed to.*\"",
  "paths_include_glob": "**/*.go"
}

// Search in YAML files only
{
  "substring_pattern": "enabled:\\s*true",
  "paths_include_glob": "**/*.yaml"
}
```

---

### 3. Code Editing Tools

#### `mcp__serena__replace_symbol_body`
**Purpose**: Replace the entire body of a symbol (function, method, type, etc.)

**Parameters**:
- `name_path` (string, required): Symbol to replace
- `relative_path` (string, required): File containing the symbol
- `body` (string, required): New symbol body (includes signature line for functions)

**Important Notes**:
- Body includes signature for functions: `func Foo() error { ... }`
- Body does NOT include docstrings/comments above the symbol
- Body does NOT include imports
- Only use when you know EXACTLY what constitutes the symbol body

**When to use**:
- Rewriting a complete function/method
- Changing a type definition
- Replacing a variable declaration

**When NOT to use**:
- ❌ Making small changes within a function → use regex-based Edit tool instead
- ❌ Adding new symbols → use insert_after_symbol or insert_before_symbol

**Example**:
```json
{
  "name_path": "Config/Validate",
  "relative_path": "internal/config/config.go",
  "body": "func (c *Config) Validate() error {\n\tif c.Name == \"\" {\n\t\treturn fmt.Errorf(\"name is required\")\n\t}\n\treturn nil\n}"
}
```

---

#### `mcp__serena__insert_after_symbol`
**Purpose**: Insert content after a symbol definition

**Parameters**:
- `name_path` (string, required): Symbol after which to insert
- `relative_path` (string, required): File containing the symbol
- `body` (string, required): Content to insert (begins on line after symbol)

**When to use**:
- Adding a new method to a type (insert after last method)
- Adding a new function (insert after related function)
- Adding content at end of file (insert after last top-level symbol)

**Example**:
```json
{
  "name_path": "Server/Start",
  "relative_path": "internal/gateway/gateway.go",
  "body": "\n// Stop stops the server gracefully\nfunc (s *Server) Stop() error {\n\treturn nil\n}\n"
}
```

---

#### `mcp__serena__insert_before_symbol`
**Purpose**: Insert content before a symbol definition

**Parameters**:
- `name_path` (string, required): Symbol before which to insert
- `relative_path` (string, required): File containing the symbol
- `body` (string, required): Content to insert

**When to use**:
- Adding imports (insert before first top-level symbol)
- Adding a new type before a function that uses it
- Inserting helper functions before main function

**Example**:
```json
{
  "name_path": "/Config",
  "relative_path": "internal/config/config.go",
  "body": "import \"fmt\"\n\n"
}
```

---

#### `mcp__serena__rename_symbol`
**Purpose**: Rename a symbol throughout the entire codebase

**Parameters**:
- `name_path` (string, required): Symbol to rename
- `relative_path` (string, required): File containing the symbol
- `new_name` (string, required): New name for the symbol

**When to use**:
- Renaming functions, types, methods, variables globally
- Refactoring for better naming conventions

**Note**: For languages with method overloading (Java), name_path may need to include signature

**Example**:
```json
{
  "name_path": "Config/Validate",
  "relative_path": "internal/config/config.go",
  "new_name": "ValidateConfig"
}
```

---

### 4. Project Memory Management

#### `mcp__serena__list_memories`
**Purpose**: List all available memory files

**Parameters**: None

**When to use**:
- Starting a new session to see what context is available
- Understanding what project knowledge exists

---

#### `mcp__serena__read_memory`
**Purpose**: Read a memory file's contents

**Parameters**:
- `memory_file_name` (string, required): Name of memory file (without .md extension)
- `max_answer_chars` (int, default: -1): Limit output length

**When to use**:
- Reading project context relevant to current task
- Understanding architecture, conventions, or status
- Only read memories relevant to your current task

**Best Practice**: Don't read the same memory multiple times in one conversation

**Example**:
```json
{
  "memory_file_name": "implementation_status"
}
```

---

#### `mcp__serena__write_memory`
**Purpose**: Write or update a memory file

**Parameters**:
- `memory_name` (string, required): Memory file name (meaningful name)
- `content` (string, required): UTF-8 encoded markdown content
- `max_answer_chars` (int, default: -1): Limit output length

**When to use**:
- After implementing significant features
- Documenting architectural decisions
- Recording project conventions or patterns
- Updating implementation status

**Best Practice**: Always update relevant memories after completing implementation work

**Example**:
```json
{
  "memory_name": "implementation_status",
  "content": "# Implementation Status\n\n## 2025-10-18: HTTP Path Configuration\n\n..."
}
```

---

#### `mcp__serena__delete_memory`
**Purpose**: Delete a memory file

**Parameters**:
- `memory_file_name` (string, required): Name of memory file to delete

**When to use**: Only when user explicitly requests it or information is obsolete

---

### 5. Meta-Cognitive Tools

These tools help you reflect on your work and ensure you're on track.

#### `mcp__serena__think_about_collected_information`
**Purpose**: Reflect on whether gathered information is sufficient and relevant

**When to use**: ALWAYS call after completing non-trivial search sequences (find_symbol, find_referencing_symbols, search_for_pattern, etc.)

---

#### `mcp__serena__think_about_task_adherence`
**Purpose**: Check if you're still on track with the task

**When to use**: ALWAYS call before inserting, replacing, or deleting code (especially in long conversations)

---

#### `mcp__serena__think_about_whether_you_are_done`
**Purpose**: Reflect on whether task is complete

**When to use**: ALWAYS call when you feel done with what the user asked for

---

### 6. Onboarding Tools

#### `mcp__serena__check_onboarding_performed`
**Purpose**: Check if project onboarding is complete

**When to use**: Beginning of conversation to verify Serena is ready

---

#### `mcp__serena__onboarding`
**Purpose**: Get onboarding instructions

**When to use**: Only if check_onboarding_performed returns that onboarding is needed

---

## Common Workflows

### Workflow 1: Understanding a New File

```
1. mcp__serena__get_symbols_overview (see top-level structure)
2. mcp__serena__find_symbol (read specific symbols with include_body=true)
3. mcp__serena__think_about_collected_information
```

### Workflow 2: Finding and Editing a Symbol

```
1. mcp__serena__find_symbol (locate the symbol, include_body=true)
2. mcp__serena__find_referencing_symbols (understand usage, if needed)
3. mcp__serena__think_about_task_adherence
4. mcp__serena__replace_symbol_body (make the change)
```

### Workflow 3: Adding a New Feature

```
1. mcp__serena__read_memory (relevant project context)
2. mcp__serena__get_symbols_overview (understand affected files)
3. mcp__serena__find_symbol (read related symbols)
4. mcp__serena__think_about_collected_information
5. mcp__serena__insert_after_symbol or mcp__serena__insert_before_symbol
6. mcp__serena__think_about_whether_you_are_done
7. mcp__serena__write_memory (update implementation_status)
```

### Workflow 4: Refactoring/Renaming

```
1. mcp__serena__find_symbol (locate the symbol)
2. mcp__serena__find_referencing_symbols (understand impact)
3. mcp__serena__think_about_collected_information
4. mcp__serena__rename_symbol (perform rename)
```

---

## Decision Trees

### "Should I read this file?"

```
START: Do you need to understand a file?
│
├─ YES: Do you know which specific symbols you need?
│  │
│  ├─ YES: Use find_symbol with include_body=true for those symbols only
│  │
│  └─ NO: Use get_symbols_overview first, then decide
│
└─ NO: Don't read it!
```

### "Which search tool should I use?"

```
START: What are you searching for?
│
├─ A function/type/method name → mcp__serena__find_symbol
│
├─ Usage/references of a symbol → mcp__serena__find_referencing_symbols
│
├─ A file name → mcp__serena__find_file
│
├─ String literals, comments, error messages → mcp__serena__search_for_pattern
│
└─ General exploration → mcp__serena__get_symbols_overview
```

### "Which edit tool should I use?"

```
START: What are you editing?
│
├─ Entire symbol body → mcp__serena__replace_symbol_body
│
├─ Adding new symbol → mcp__serena__insert_after_symbol or insert_before_symbol
│
├─ Renaming a symbol → mcp__serena__rename_symbol
│
└─ Small change within symbol → Use standard Edit tool (regex-based)
```

---

## Best Practices Summary

### ✅ DO:
- Start with `get_symbols_overview` when exploring new files
- Use symbolic tools to read only what you need
- Call meta-cognitive tools after search/before edit/when done
- Update memories after implementing features
- Use `relative_path` parameter to restrict searches when possible
- Use `include_body=false` to see signatures before reading implementations
- Use `depth=1` to see a type's methods without their bodies

### ❌ DON'T:
- Read entire files without checking symbols overview first
- Use pattern search for things that have symbols (functions, types)
- Read the same content multiple times with different tools
- Forget to update memories after significant changes
- Use replace_symbol_body for small changes within a function
- Skip meta-cognitive tools in complex workflows

---

## Integration with Standard Tools

Serena tools complement standard Claude Code tools:

- **File operations**: Use standard Read/Write/Edit for simple file operations
- **Symbol operations**: Use Serena tools for code-aware operations
- **Search**: Use Serena find_symbol for code, Grep for generic text search
- **Navigation**: Use Serena get_symbols_overview instead of reading full files
- **Memory**: Use Serena memories for project context across sessions

---

## Performance Tips

1. **Minimize reads**: Every file read consumes tokens. Use symbolic tools to read less.
2. **Use relative_path**: Restricting searches to specific files/dirs is much faster.
3. **Check overview first**: `get_symbols_overview` is cheap and informative.
4. **Include bodies only when needed**: Signatures are often enough for understanding.
5. **Use depth wisely**: depth=0 for single symbol, depth=1 for methods, rarely go deeper.

---

This guide should be consulted whenever choosing between tools or planning a code navigation/editing workflow.
