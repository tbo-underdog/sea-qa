# SEA-QA — Shift‑Left Endpoint Assurance

A fast, language‑agnostic **API test runner** with **OpenAPI contract validation**, **coverage**, and **CI‑ready** reports. SEA‑QA helps you shift API quality left: catch contract drift, enforce response schemas, and ship with confidence straight from CI.

---

## Why SEA‑QA?

- 🔧 **Language‑agnostic**: YAML suites drive HTTP requests & assertions
- 📜 **Contract checks**: Validate responses against OpenAPI (status, headers, body schema)
- 📈 **Coverage**: See which paths/methods in your spec were exercised
- ⚡ **Parallel**: Run scenarios concurrently; **fail‑fast** when you need it
- 🪝 **Hooks**: Pre/after process hooks via simple stdin/stdout JSON (inject tokens, redact, mutate)
- 📦 **CI‑first**: JUnit XML, JSON, **HTML** report artifacts; easy to wire into Jenkins, CircleCI, GitHub Actions

---

## Quickstart

### 1) Build the CLI

```bash
go build -o seaqa ./cmd/sea-qa
```

> Requires Go 1.21+ (tested with 1.22).

### 2) Minimal env

`tests/examples/jsonplaceholder/env.json`

```json
{
  "BASE_URL": "https://jsonplaceholder.typicode.com"
}
```

### 3) Example suite

`tests/examples/jsonplaceholder/suite.yaml`

```yaml
name: JSONPlaceholder — SEA-QA Demo
openapi: tests/examples/jsonplaceholder/openapi.json

scenarios:
  - name: List posts (GET /posts?userId=1)
    tags: [stable, posts]
    steps:
      - request:
          method: GET
          url: ${BASE_URL}/posts?userId=1
          headers: { Accept: application/json }
        expect:
          - type: status
            value: 200
          - type: contract
            value: true

  - name: Get post 1 (GET /posts/1)
    tags: [stable, posts]
    steps:
      - request:
          method: GET
          url: ${BASE_URL}/posts/1
          headers: { Accept: application/json }
        expect:
          - type: status
            value: 200
          - type: jsonPath
            target: $.id
            value: 1
          - type: contract
            value: true
```

### 4) Run

```bash
./seaqa \
  --spec tests/examples/jsonplaceholder/suite.yaml \
  --env  tests/examples/jsonplaceholder/env.json \
  --openapi tests/examples/jsonplaceholder/openapi.json \
  --out reports -v --parallel 4
```

Artifacts:

- `reports/report.html`
- `reports/results.json`
- `reports/junit.xml`
- `reports/coverage.json` (when `--openapi` used)

---

## Test Suite Format

```yaml
name: My API Suite
openapi: path/to/openapi.yaml   # optional (can also pass via --openapi)

scenarios:
  - name: Create and fetch widget
    tags: [smoke, widgets]
    setup:        # optional (list of actions)
      - request: { method: POST, url: ${BASE_URL}/reset }
    steps:
      - request:
          method: POST
          url: ${BASE_URL}/widgets
          headers: { Content-Type: application/json, Accept: application/json }
          body:
            name: "SEA-QA"
            tier: "free"
          timeout_ms: 8000
        expect:
          - type: status
            value: 201
          - type: contract
            value: true
      - request:
          method: GET
          url: ${BASE_URL}/widgets/1
          headers: { Accept: application/json }
        expect:
          - type: status
            value: 200
          - type: jsonPath
            target: $.name
            value: "SEA-QA"
    teardown:     # optional
      - request: { method: DELETE, url: ${BASE_URL}/widgets/1 }
```

- Variables: `${KEY}` from `--env` JSON files (merged left→right).
- **Timeout field:** `timeout_ms` (snake_case) is the **only** supported key.
- Tag filtering: `--include-tags smoke` or `--exclude-tags flaky`.

---

## Expectations

- `status` — exact integer HTTP status
- `jsonPath` — basic top‑level JSON path `$.field` equality
- `contract` — validate response against OpenAPI (status, headers, schema)

Example:

```yaml
expect:
  - type: status
    value: 200
  - type: jsonPath
    target: $.id
    value: 1
  - type: contract
    value: true
```

> JSONPath support is intentionally minimal in v1 (top‑level only). Deeper paths/arrays land in the roadmap.

---

## OpenAPI Contract Validation

When `--openapi` (or `openapi:` in the suite) is provided, SEA‑QA:

1. Routes the request to a matching path+method in the spec
2. Validates status code, headers, and body schema
3. Records coverage for the matched route

SEA‑QA is **strict**: malformed specs fail fast. This keeps your source of truth clean.

---

## Coverage

Coverage is emitted to `reports/coverage.json` and includes:

- `matched`: endpoints exercised during the run
- `total`: total operations in the spec
- `percent`: matched / total * 100

Gate builds on coverage:

```bash
./seaqa --spec ... --openapi ... --coverage-min 70
```

---

## Hooks

Run small processes **before/after** a step to inject tokens, mutate requests, or collect telemetry.

### Step definition

```yaml
steps:
  - request: { method: GET, url: ${BASE_URL}/widgets }
    hooks:
      - when: before
        process: ["scripts/gettoken/gettoken"]   # your helper binary/script
      - when: after
        process: ["scripts/scrub-logs"]          # optional post-processing
```

### Hook protocol

SEA‑QA sends JSON on stdin and expects JSON on stdout:

**Input**

```json
{
  "vars": { "BASE_URL": "..." },
  "request": { "method": "GET", "url": "..." }
}
```

**Output**

```json
{
  "vars": { "TOKEN": "abc" },
  "request": { "headers": { "Authorization": "Bearer abc" } },
  "errors": []
}
```

Any `errors` emitted by a hook will fail the step and be printed in reports.

> Example helper scripts live under `scripts/` — each script has its own folder and a small `main.go`.

---

## Command Line

```
seaqa --spec <suite.yaml> [flags]

  --env <file1.json[,file2.json,...]>   Load variables (merged left→right)
  --openapi <file>                      Validate against this OpenAPI
  --out <dir>                           Output directory (default: reports)
  --parallel <N>                        Run scenarios concurrently (default: 1)
  --fail-fast                           Stop after first failing scenario
  --include-tags <t1,t2>                Only run scenarios with these tags (OR)
  --exclude-tags <t1,t2>                Skip scenarios with these tags (OR)
  --coverage-min <percent>              Fail if coverage below threshold
  --json / --junit / --html             Toggle artifact formats (default: all)
  -v                                    Verbose failure printing to stderr
```

---

## CI Examples

### Jenkins (Declarative Pipeline)

`Jenkinsfile`:

```groovy
pipeline {
  agent any

  options {
    timestamps()
    ansiColor('xterm')
  }

  environment {
    GO111MODULE = 'on'
    GOCACHE     = "${WORKSPACE}@tmp/go-build"
    GOMODCACHE  = "${WORKSPACE}@tmp/go-mod"
    SPEC        = 'tests/examples/jsonplaceholder/suite.yaml'
    OPENAPI     = 'tests/examples/jsonplaceholder/openapi.json'
    ENVFILE     = 'tests/examples/jsonplaceholder/env.json'
    OUTDIR      = 'reports'
  }

  stages {
    stage('Checkout') { steps { checkout scm } }

    stage('Build CLI') {
      steps {
        sh 'go mod download'
        sh 'go build -o seaqa ./cmd/sea-qa'
      }
    }

    stage('Run SEA-QA') {
      steps {
        sh '''
          ./seaqa \
            --spec "${SPEC}" \
            --openapi "${OPENAPI}" \
            --env "${ENVFILE}" \
            --out "${OUTDIR}" \
            --parallel 4 -v
        '''
      }
    }
  }

  post {
    always {
      junit allowEmptyResults: true, testResults: 'reports/junit.xml'
      archiveArtifacts artifacts: 'reports/**', fingerprint: true
      // With the "HTML Publisher" plugin:
      // publishHTML(target: [reportDir: 'reports', reportFiles: 'report.html', reportName: 'SEA-QA Report', keepAll: true])
    }
  }
}
```

> On Windows agents replace `sh` with `bat` and `./seaqa` with `seaqa.exe`.

### CircleCI

`.circleci/config.yml`:

```yaml
version: 2.1

jobs:
  seaqa:
    docker:
      - image: cimg/go:1.22
    environment:
      SPEC: tests/examples/jsonplaceholder/suite.yaml
      OPENAPI: tests/examples/jsonplaceholder/openapi.json
      ENVFILE: tests/examples/jsonplaceholder/env.json
      OUTDIR: reports
    steps:
      - checkout
      - restore_cache:
          keys:
            - go-mod-{{ checksum "go.sum" }}
            - go-mod-
      - run: go mod download
      - save_cache:
          key: go-mod-{{ checksum "go.sum" }}
          paths: [~/go/pkg/mod]
      - run: go build -o seaqa ./cmd/sea-qa
      - run: |
          ./seaqa \
            --spec "$SPEC" \
            --openapi "$OPENAPI" \
            --env "$ENVFILE" \
            --out "$OUTDIR" \
            --parallel 4 -v
      - store_test_results: { path: reports }
      - store_artifacts: { path: reports, destination: reports }

workflows:
  seaqa:
    jobs: [seaqa]
```

### GitHub Actions

`.github/workflows/seaqa.yml`:

```yaml
name: SEA-QA

on:
  push:
    branches: [ main, master ]
  pull_request:

jobs:
  run:
    runs-on: ubuntu-latest
    env:
      SPEC: tests/examples/jsonplaceholder/suite.yaml
      OPENAPI: tests/examples/jsonplaceholder/openapi.json
      ENVFILE: tests/examples/jsonplaceholder/env.json
      OUTDIR: reports
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - uses: actions/cache@v4
        with:
          path: |
            ~/go/pkg/mod
            ~/.cache/go-build
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-
      - run: go build -o seaqa ./cmd/sea-qa
      - run: |
          ./seaqa \
            --spec "$SPEC" \
            --openapi "$OPENAPI" \
            --env "$ENVFILE" \
            --out "$OUTDIR" \
            --parallel 4 -v
      - if: always()
        uses: actions/upload-artifact@v4
        with:
          name: seaqa-reports
          path: reports/
```

---

## Troubleshooting

- **`decode: unknown field "timeoutMs"`** → use `timeout_ms`
- **`unresolved variables in URL: ${FOO}`** → pass `--env` JSON defining `FOO`
- **`contract: route not found`** → request doesn’t match any path+method; check base URL, path, and OpenAPI spec
- **`contract: failed to decode response body`** → server returned non‑JSON for a JSON schema; validate the endpoint or adjust expectations

---

## Contributing

PRs welcome! Before submitting:

1. `go fmt ./... && go test ./...`
2. Update/add fixtures under `tests/examples/*` as needed
3. Update README/CLI docs if your change affects behavior

---

## License

**Apache License 2.0** — see [`LICENSE`](LICENSE) for full text.
