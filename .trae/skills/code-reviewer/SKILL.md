---
name: "code-reviewer"
description: "Reviews Go code for best practices, bugs, concurrency safety, and business logic. Invoke when user asks for code review or after completing significant code changes."
---

# Code Reviewer

This skill performs comprehensive code review for Go projects following the project's conventions and best practices.

## When to Invoke

- User asks for code review
- After completing significant code changes
- Before merging code changes
- User wants to check code quality

## Review Checklist

### 1. Code Style and Format

- [ ] Are variable, function, and type names clear and follow Go conventions?
- [ ] Are there unnecessary code comments or missing critical comments?
- [ ] Is the code formatted with `gofmt`?

### 2. Error Handling

- [ ] Are errors properly checked (`if err != nil`)?
- [ ] Do error messages have sufficient context (e.g., `fmt.Errorf("xxx: %v", err)`)?
- [ ] Are `panic` and `recover` used appropriately (avoid abuse)?

### 3. Concurrency Safety

- [ ] Is access to shared variables protected with mutex (`sync.Mutex`) or atomic operations?
- [ ] Are channels used correctly for goroutine synchronization?
- [ ] Are there potential deadlocks, race conditions, or goroutine leaks?
- [ ] Are `sync.WaitGroup` and `context.Context` used properly for concurrency control?

### 4. Performance and Resource Management

- [ ] Are there unnecessary memory allocations (e.g., string concatenation with `+` instead of `strings.Builder`)?
- [ ] Are resources properly closed (files, network connections, HTTP response bodies)? Use `defer` when appropriate.
- [ ] Can repeated calculations inside loops be extracted outside?
- [ ] Can `sync.Pool` be used to reduce object allocation?

### 5. Code Organization and Readability

- [ ] Are functions too long? Can they be split into smaller functions?
- [ ] Is the "early return" principle followed to reduce nesting?
- [ ] Is the package structure clear with single responsibility?
- [ ] Are global variables and package-level state used appropriately?

### 6. Testing

- [ ] Are errors in tests handled correctly (use `t.Fatal` instead of `panic`)?

### 7. Common Pitfalls

- [ ] Is `defer` used inside loops? (May cause delayed resource release, consider anonymous functions)
- [ ] Are non-copyable types like `sync.Mutex`, `sync.WaitGroup` incorrectly copied?
- [ ] Is `time.After` used incorrectly causing memory leaks?

### 8. Business Logic Checks

- [ ] Are client request message fields fully validated?
  - Integer fields: negative values, excessively large values
  - Arrays: empty check
- [ ] Numeric overflow issues: Can int32 arithmetic operations exceed max/min values?
- [ ] Division by zero: Can the denominator be zero?
- [ ] Purchase/exchange logic: Deduct costs first, then deliver items
- [ ] Array index out of bounds risks
- [ ] Nil pointer dereference risks

## Review Process

1. **Read the code file(s)** to understand the implementation
2. **Check each item** in the review checklist
3. **Identify issues** and categorize by severity:
   - **Critical**: Security vulnerabilities, data corruption, crashes
   - **Major**: Logic errors, resource leaks, concurrency issues
   - **Minor**: Code style, performance optimizations, readability
4. **Provide feedback** with:
   - Issue description
   - File location and line numbers
   - Suggested fix or improvement
   - Code example when applicable

## Output Format

```markdown
## Code Review Report

### Summary
Brief overview of the code quality and main findings.

### Critical Issues
1. [File:Line] Issue description
   - Suggested fix: ...

### Major Issues
1. [File:Line] Issue description
   - Suggested fix: ...

### Minor Issues
1. [File:Line] Issue description
   - Suggested fix: ...

### Suggestions
- Improvement suggestions that are not bugs but could enhance code quality.

### Positive Aspects
- Highlight good practices found in the code.
```

## Example Issues

### Nil Pointer Check

```go
// Bad
hero := bagHero.GetElem(req.GetId())
hero.Level += 1  // Potential nil pointer dereference

// Good
hero := bagHero.GetElem(req.GetId())
if hero == nil {
    l.Error("hero not found", "heroId", req.GetId())
    return nil, errors.New("heroNotFound")
}
hero.Level += 1
```

### Integer Overflow

```go
// Bad
total := a + b  // May overflow int32

// Good
if int64(a) + int64(b) > math.MaxInt32 {
    return errors.New("overflow")
}
total := a + b
```

### Resource Management

```go
// Bad
file, _ := os.Open(path)
data, _ := io.ReadAll(file)

// Good
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close()
data, err := io.ReadAll(file)
if err != nil {
    return err
}
```

### Business Logic Order

```go
// Bad: Deliver first, then deduct (may cause exploitation)
bag.AddItem(item)
bag.DelItems(cost)

// Good: Deduct first, then deliver
if !bag.IsEnough(cost) {
    return errors.New("notEnough")
}
bag.DelItems(cost)
bag.AddItem(item)
```

## Notes

- Focus on actionable feedback
- Prioritize security and correctness over style
- Consider the project's existing patterns and conventions
- Provide context for why something is an issue
- Suggest concrete solutions, not just problems
