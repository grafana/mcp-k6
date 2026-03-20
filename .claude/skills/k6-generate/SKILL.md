---
description: Generate a production-ready k6 script following official best practices
argument-hint: describe what you want to test (e.g., "load test my checkout API for 100 users")
---

Generate a k6 performance test script for: $ARGUMENTS

## Instructions

### Step 1: Check for mcp-k6

If the tools `validate_script`, `run_script`, `list_sections`, and `get_documentation` are available:
1. Read the `docs://k6/best_practices` resource for current guidelines.
2. Use `list_sections` + `get_documentation` to look up relevant k6 docs (scenarios, thresholds, the specific protocol you're testing).
3. Generate the script following the workflow in Step 2 below, informed by what you found in the docs.
4. Use `validate_script` to verify the script runs cleanly (1 VU, 1 iteration) and fix any errors.
5. Offer to run with `run_script` and suggest appropriate VU/duration parameters.

If those tools are **not** available, follow the workflow below directly using the embedded best practices.

---

### Step 2: Generate the script

Apply the following best practices:

@resources/best_practices.md

---

### Step 3: Save the script

1. Create the directory `k6/scripts/` if it does not exist.
2. Choose a descriptive filename in `lowercase-kebab-case.js` (e.g. `checkout-load-test.js`).
3. Write the complete script to `k6/scripts/<filename>.js`.
4. Confirm the file path in your response.

---

## Output format

Present your response as:

1. **Script overview** — what the script tests and why the chosen executor/shape fits
2. **Best practices applied** — 3–5 key decisions made for this specific script
3. **Generated script** — complete, runnable k6 script with comments
4. **Script location** — full path where the file was saved
5. **Validation results** — output from `validate_script` (if mcp-k6 available) or manual checklist
6. **Next steps** — suggested `run_script` parameters or `k6 run` command with recommended VUs/duration
