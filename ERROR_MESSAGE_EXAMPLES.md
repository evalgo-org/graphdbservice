# Improved JSON Error Messages - Examples

This document shows examples of the improved error messages that users will see in the Web UI when they submit invalid JSON.

## Overview

The error handling has been significantly enhanced to provide:
- **Precise error location** (line and column numbers)
- **Visual code snippets** showing exactly where the error occurred
- **Helpful hints** explaining common mistakes and how to fix them
- **User-friendly explanations** instead of cryptic parser errors

## Example Error Messages

### 1. Missing Closing Brace

**Invalid JSON:**
```json
{"version": "v0.0.1", "tasks": [
```

**Error Message Displayed:**
```
Invalid JSON: unexpected end of JSON input

Hint: The JSON is incomplete. Check for missing closing brackets '}' or ']'
```

---

### 2. Trailing Comma

**Invalid JSON:**
```json
{"version": "v0.0.1", "tasks": [],}
```

**Error Message Displayed:**
```
Invalid JSON: invalid character '}' looking for beginning of object key

Hint: Check for trailing commas or missing values
```

---

### 3. Syntax Error with Position

**Invalid JSON:**
```json
{
  "version": "v0.0.1"
  "tasks": []
}
```

**Error Message Displayed:**
```
Invalid JSON: invalid character '"' after object key:value pair at line 3, column 3

  "tasks": []
  ^

Hint: Check for missing commas, quotes, brackets, or trailing commas
```

---

### 4. Missing Quotes on Property Name

**Invalid JSON:**
```json
{version: "v0.0.1"}
```

**Error Message Displayed:**
```
Invalid JSON: invalid character 'v' looking for beginning of object key

Hint: Look for special characters, unescaped quotes, or formatting issues
```

---

### 5. Type Mismatch Error

**Invalid JSON:**
```json
{"version": 123, "tasks": []}
```

**Error Message Displayed:**
```
Invalid JSON: Type mismatch in field 'version': expected string but got number

Hint: Check that field values match the expected data type (string, number, object, array)
```

---

### 6. Missing Colon After Property Name

**Invalid JSON:**
```json
{"version" "v0.0.1"}
```

**Error Message Displayed:**
```
Invalid JSON: invalid character '"' after object key

Hint: Check for missing colon ':' after a property name, or missing comma ',' between properties
```

---

## Implementation Details

### Location in Code
- **Handler**: `cmd/web_handlers.go:37` (`uiExecuteHandler`)
- **Error Formatter**: `cmd/web_handlers.go:31` (`formatJSONError`)
- **Location Helper**: `cmd/web_handlers.go:72` (`getErrorLocation`)

### Features
1. **Error Type Detection**: Distinguishes between syntax errors, type errors, and other JSON parsing errors
2. **Context Snippets**: Shows the actual line of code where the error occurred
3. **Visual Pointer**: Adds a `^` character pointing to the exact error position
4. **Contextual Hints**: Provides specific advice based on the error type
5. **HTML Formatting**: Uses `<br>`, `<code>`, and `<small>` tags for better readability in the UI

### Test Coverage
- **Unit Tests**: `cmd/web_handlers_test.go`
  - `TestFormatJSONError` - Tests the error formatting logic
  - `TestUIExecuteHandlerJSONErrors` - Tests the full UI handler with various error scenarios
  - `TestGetErrorLocation` - Tests the line/column calculation logic

All tests pass successfully ✓

---

## User Experience Improvements

### Before
```
Invalid JSON format: invalid character '}' looking for beginning of object key
```

### After
```
Invalid JSON: invalid character '}' looking for beginning of object key

Hint: Check for trailing commas or missing values
```

The new error messages are:
- ✓ More informative
- ✓ Easier to understand for non-developers
- ✓ Include actionable hints
- ✓ Show exact error locations
- ✓ Display code context when available

This significantly improves the user experience, especially for users who may not be familiar with JSON syntax.
