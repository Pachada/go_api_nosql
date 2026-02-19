# Go Coding Style Guide

These rules are designed to guide the generation of Go code that is simple, readable, and maintainable, adhering to Go's
idiomatic style and the principles of pragmatic engineering.

## 1. The Principle of Least Abstraction

Your primary goal is clarity, not cleverness. Start with the simplest possible solution.

- **Rule 1.1: Default to a Single Function** - Solve the problem within a single function first. Do not create helper functions, new types, or new packages prematurely.

- **Rule 1.2: Justify Every Abstraction** - Before creating a new function, struct, or package, you must justify its existence based on the rules below (e.g., function length, parameter count, or the Rule of Three). If there's no strong reason to abstract, don't.

## 2. Function Design and Granularity

Functions are the fundamental building blocks. They must be clear and focused.

- **Rule 2.1: Functions Do One Thing** - Every function should have a single, clear responsibility. If you cannot describe what a function does in one simple sentence, it's doing too much.

- **Rule 2.2: Strict Function Length Limit** - A function should rarely exceed 50 lines. If a function grows longer, immediately decompose it into smaller, private helper functions. Keep these helpers in the same file to maintain locality.

- **Rule 2.3: Strict Parameter Limit** - A function must not have more than four parameters.
    - If you need more, group related parameters into a struct.
    - If a function needs to operate on shared state, make it a method on a struct that holds that state. This is preferable to passing the state through multiple function parameters.

- **Rule 2.4: Return Values** - Return one or two values directly. If you need to return three or more related values, use a named struct to give them context and clarity. Avoid returning a map or a bare tuple of many values.

## 3. Duplication vs. Abstraction

Avoid hasty abstractions. Duplication is often better than the wrong abstraction.

- **Rule 3.1: The Rule of Three** - Do not refactor duplicated code on its first or second appearance. Only when you encounter the third instance should you consider creating a shared abstraction (like a new function).

- **Rule 3.2: Verify True Duplication** - Before refactoring, confirm the duplicated code represents the same core logic. If the code blocks look similar by coincidence but handle different business rules that might change independently, they must remain separate. Creating an abstraction here would create a tightly coupled but logically unrelated dependency.

## 4. Package and Interface Philosophy

Follow Go's idiomatic approach to packages and interfaces.

- **Rule 4.1: Packages Have a Singular Purpose** - A package should represent a single concept (e.g., `http`, `user`, `models`). Do not create generic "utility," "common," or "helpers" packages. Keep related types and functions together in a cohesive package.

- **Rule 4.2: Interfaces are Defined by the Consumer** - Do not define large, monolithic interfaces on the producer side. Instead, the function that uses a dependency should define a small interface describing only the behavior it requires. This follows the Go proverb: "The bigger the interface, the weaker the abstraction."

- **Rule 4.3: Keep Interfaces Small** - An interface should ideally have only one method. Interfaces with more than three methods are a red flag and should be re-evaluated.
