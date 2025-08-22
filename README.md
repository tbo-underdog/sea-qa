# sea-qa

**Contract-first API testing for CI.**  
sea-qa runs black-box API tests from a compact YAML/JSON format, validates responses, **enforces OpenAPI 3 contracts strictly**, and emits machine- and human-readable reports. It’s a single binary (or Docker image) built to slot cleanly into GitHub Actions, CircleCI, Jenkins, or any CI.

---

## Why sea-qa?

- **Stop contract drift** — fail PRs when responses don’t match your OpenAPI.  
- **CI-first** — no UI lock-in; just a fast binary with clear exit codes & reports.  
- **Auditable** — tests-as-code, easy to diff/review, reproducible runs.  
- **Fast** — parallel scenarios, pooled HTTP, HTTP/2.

---

## Features

- 🧪 Spec → IR → **Runner** pipeline (language-agnostic tests)
- ⚖️ Assertions: **status**, simple **JSONPath** (`$.field` top-level), **OpenAPI contract** per step
- 📘 **Strict OpenAPI 3** validation + **coverage** (JSON) + optional coverage gate
- 🚦 **Fail-fast** and **parallel** scenarios (`--parallel N`, `--fail-fast`)
- 🏷️ **Tag filters** (`--include-tags`, `--exclude-tags`)
- 📊 Reports: **JSON**, **JUnit XML**, **HTML**
- 🔍 **Contract diff** between two OpenAPI specs
- 🧰 **Process hooks** (stdin/stdout JSON) for tiny pre/post tasks
- 🐳 Lightweight **Docker** image & CI examples

---

## Quickstart

### 1) Install

**Go (local):**
```bash
go build ./cmd/sea-qa
# => ./sea-qa

2) Minimal env file

tests/examples/jsonplaceholder/env.json