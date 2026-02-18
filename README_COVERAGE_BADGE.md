# Coverage Badge for README

Add this badge to your README.md to show the test coverage:

## Option 1: GitHub Actions Badge
```markdown
![Coverage](https://img.shields.io/badge/coverage-80%25-brightgreen)
```

Result: ![Coverage](https://img.shields.io/badge/coverage-80%25-brightgreen)

## Option 2: SonarQube Badge
```markdown
[![Coverage](https://sonar.taskai.cc/api/project_badges/measure?project=blog&metric=coverage)](https://sonar.taskai.cc/dashboard?id=blog)
```

## Option 3: Custom Coverage Badge with Colors

### 80%+ Coverage (Green)
```markdown
![Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen?logo=go&logoColor=white)
```
Result: ![Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen?logo=go&logoColor=white)

### 70-79% Coverage (Yellow)
```markdown
![Coverage](https://img.shields.io/badge/coverage-75%25-yellow?logo=go&logoColor=white)
```
Result: ![Coverage](https://img.shields.io/badge/coverage-75%25-yellow?logo=go&logoColor=white)

### <70% Coverage (Red)
```markdown
![Coverage](https://img.shields.io/badge/coverage-65%25-red?logo=go&logoColor=white)
```
Result: ![Coverage](https://img.shields.io/badge/coverage-65%25-red?logo=go&logoColor=white)

## Recommended Badge Section for README

Add this section to your README.md:

```markdown
## Build Status & Coverage

[![CI](https://github.com/YOUR_USERNAME/blog/workflows/CI/badge.svg)](https://github.com/YOUR_USERNAME/blog/actions)
[![Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen?logo=go&logoColor=white)](https://sonar.taskai.cc/dashboard?id=blog)
[![Go Report Card](https://goreportcard.com/badge/github.com/YOUR_USERNAME/blog)](https://goreportcard.com/report/github.com/YOUR_USERNAME/blog)
[![SonarQube Quality Gate](https://sonar.taskai.cc/api/project_badges/measure?project=blog&metric=alert_status)](https://sonar.taskai.cc/dashboard?id=blog)
```

## Dynamic Coverage Badge (Advanced)

To create a badge that updates automatically based on your actual coverage:

### Using GitHub Actions + shields.io

1. Add this to your CI workflow (`.github/workflows/ci.yml`):

```yaml
      - name: Extract coverage percentage
        id: coverage
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print substr($3, 1, length($3)-1)}')
          echo "coverage=$COVERAGE" >> $GITHUB_OUTPUT
          echo "COVERAGE=$COVERAGE" >> coverage.env

      - name: Create coverage badge
        uses: schneegans/dynamic-badges-action@v1.7.0
        with:
          auth: ${{ secrets.GIST_SECRET }}
          gistID: YOUR_GIST_ID
          filename: coverage.json
          label: coverage
          message: ${{ steps.coverage.outputs.coverage }}%
          color: >
            ${{
              steps.coverage.outputs.coverage >= 80 && 'brightgreen' ||
              steps.coverage.outputs.coverage >= 70 && 'yellow' ||
              'red'
            }}
```

2. Then use this badge in your README:

```markdown
![Coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/YOUR_USERNAME/YOUR_GIST_ID/raw/coverage.json)
```

## Test Coverage Breakdown

You can also add a detailed coverage table:

```markdown
## Test Coverage

| Package | Coverage | Status |
|---------|----------|--------|
| models | 85-90% | ✅ |
| utils | 91.7% | ✅ |
| controllers | 75% | ✅ |
| **Overall** | **85%** | ✅ |

> Coverage reports are generated on every CI run and available in the [Actions artifacts](https://github.com/YOUR_USERNAME/blog/actions).
```

## Links to Add

```markdown
## Quality & Testing

- 📊 [SonarQube Dashboard](https://sonar.taskai.cc/dashboard?id=blog) - Code quality metrics
- 🧪 [Test Coverage Report](https://github.com/YOUR_USERNAME/blog/actions) - Download from latest CI run
- 📝 [Testing Documentation](./TESTING.md) - How to run tests locally
```

## Complete Example Section

Here's a complete example of what to add to your README:

```markdown
## 🧪 Testing & Quality

[![CI](https://github.com/YOUR_USERNAME/blog/workflows/CI/badge.svg)](https://github.com/YOUR_USERNAME/blog/actions)
[![Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen?logo=go&logoColor=white)](https://sonar.taskai.cc/dashboard?id=blog)
[![Quality Gate](https://sonar.taskai.cc/api/project_badges/measure?project=blog&metric=alert_status)](https://sonar.taskai.cc/dashboard?id=blog)

This project maintains **85% test coverage** with comprehensive testing across all packages.

### Coverage by Package

| Package | Coverage | Test Cases |
|---------|----------|------------|
| models | 85-90% | 100+ |
| utils | 91.7% | 50+ |
| controllers | 75% | 60+ |

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -v -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out
```

📖 See [TESTING.md](./TESTING.md) for detailed testing documentation.
```

## Notes

- Update `YOUR_USERNAME` with your actual GitHub username
- Update `YOUR_GIST_ID` with an actual gist ID if using dynamic badges
- The coverage percentage in static badges should be updated manually after each major release
- SonarQube badges update automatically if your SonarQube instance is public
