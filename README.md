# randrange

![Code Base: AI Vibes](https://img.shields.io/badge/Code%20Base-AI%20Vibes%20%F0%9F%A4%A0-blue)


A tiny, zero-dependency Go utility that prints cryptographically-strong random numbers to stdout.

## Disclaimer

This program was vibe-coded by `Anthropic Claude Opus`. As such, the author can't be held responsible for incorrect calculations. Please verify the results for any critical applications.

## Install

```bash
go install -ldflags="-s -w" github.com/yourname/randrange@latest
```

Or clone the repo and:

```bash
go build -ldflags="-s -w" -o randrange
```

## Usage

```bash
randrange [flags]
```

Flags:

| Flag             | Default | Meaning |
|------------------|---------|---------|
| `-start`         | 0       | Inclusive lower bound |
| `-end`           | 100     | Inclusive upper bound |
| `-count`         | 5       | How many numbers to emit |
| `-floats`        | false   | Generate floats (8-decimal precision) instead of integers |
| `-skew`          |         | Skew distribution: `low` (toward start) or `high` (toward end) |
| `-skew-strength` | 2.0     | Skew intensity: 1.0 = uniform, higher = more skewed |
| `-outliers`      | 0       | Number of outlier values generated near range edges |
| `-outlier-percent`| 10     | Edge zone size as percent of range (1-49) |

## Examples

Integers 0-100:
```bash
$ randrange
42
7
99
0
73
```

Floats between -1 and 1:
```bash
$ randrange -floats -start -1 -end 1 -count 3
-0.12345678
0.87654321
0.00001234
```

Only 1 integer from 10 to 20:
```bash
$ randrange -start 10 -end 20 -count 1
15
```

Integers 1-100, skewed toward low values:
```bash
$ randrange -start 1 -end 100 -count 10 -skew low
5
13
2
31
4
21
9
37
11
4
```

Floats 0-1000, strongly skewed toward high values:
```bash
$ randrange -start 0 -end 1000 -count 5 -floats -skew high -skew-strength 4.0
873.17949292
913.43629443
958.50046793
794.20765032
999.35220846
```

## Progress Status

On macOS and BSD systems, pressing **Ctrl-T** while `randrange` is running sends a `SIGINFO` signal, which causes it to print the current progress to stderr:

```
progress: 500000 / 1000000 (50.0%)
```

This is useful when generating a large quantity of numbers (e.g., `-count 1000000`) and you want to check how far along the run is. On other platforms this signal is not available and Ctrl-T has no effect.

## Outliers

The `-outliers` flag injects a fixed number of values drawn from the edges of the range, scattered randomly among the normal output. This is useful for stress-testing systems that should handle extreme values gracefully.

`-outlier-percent` controls how wide the edge zones are. With the default of `10`, the low zone is the bottom 10% of the range and the high zone is the top 10%. Each outlier is randomly assigned to either zone with a coin flip.

```bash
# 100 normal values plus 5 outliers from the edges of [0, 1000]
$ randrange -start 0 -end 1000 -count 100 -outliers 5

# Narrower 5% edge zones: outliers come from [0,50] or [950,1000]
$ randrange -start 0 -end 1000 -count 100 -outliers 5 -outlier-percent 5

# Works with floats
$ randrange -start 0 -end 1000 -count 100 -outliers 5 -floats

# Works alongside skew (normal values are skewed, outliers are uniform in edge zones)
$ randrange -start 0 -end 1000 -count 100 -outliers 5 -skew low
```

The total number of output lines is always `count + outliers`. The outlier values themselves are generated with uniform distribution (no skew applied) so they cover the full edge zone evenly. The feature is streaming — only the outlier positions are held in memory, not the values — so it works with arbitrarily large counts.

**Validation rules:**
* `-outliers` must be >= 0
* `-outlier-percent` must be between 1 and 49
* `-outlier-percent` requires `-outliers` to be set

## Skew

The `-skew` flag uses a power-law transformation to bias the distribution while keeping values random. Every value in the range can still appear, but some become more probable.

| `-skew-strength` | Effect |
|-------------------|--------|
| 1.0 | Uniform (no skew) |
| 2.0 | Moderate bias (default) |
| 3.0 | Strong clustering toward the chosen end |
| 5.0+ | Very aggressive — most values packed near one end |

## Notes

* Uses `crypto/rand` for cryptographic strength.
* Floats are rounded to 8 decimal places.
* No external dependencies; plain `go run` or `go build` is enough.

## Testing

Run the test suite with:

```bash
go test -v ./...
```

The test harness in `main_test.go` includes 30 tests split across three categories:

### Deterministic Tests

These use a pre-loaded `randSource` with known uint64 values so results are fully reproducible with no randomness involved.

* **`applySkew`** — verifies the power-law transform math directly:
  * Identity: strength `1.0` returns input unchanged for both `low` and `high`
  * Boundaries: inputs `0.0` and `1.0` always map to `0.0` and `1.0`
  * Known values: hand-computed results (e.g., `0.5^2 = 0.25`)
  * Symmetry: `applySkew(x, "low", s) + applySkew(1-x, "high", s) = 1.0`
  * Monotonicity: output preserves input ordering
  * No-skew passthrough: empty string returns input at any strength

* **`randomInt` / `randomFloat`** — verifies range and error handling:
  * Crafted inputs that map to range extremes stay within bounds
  * `min == max` returns that single value
  * `min > max` returns an error
  * Every float result equals itself when re-rounded to 8 decimal places

### Probabilistic Tests

These use real `crypto/rand` with large sample sizes (100k–500k) and generous tolerances to verify statistical properties of the actual pipeline.

* **Uniform distribution**: 500k values bucketed into 10 bins; each bin must be within 10% of the expected count (both ints and floats)
* **Skew median shift**: with `skew=low, strength=2.0` the median of 500k samples must fall well below the range midpoint (and vice versa for `skew=high`)
* **Strength comparison**: `strength=5.0` produces a more extreme median than `strength=2.0`
* **Low/high symmetry**: the fraction of `skew=low` values in the bottom half of the range must match the fraction of `skew=high` values in the top half (within 2%)
* **Bounds enforcement**: 100k samples across 5 skew configurations all stay within `[min, max]` for ints and `[min, max)` for floats
* **Full coverage**: for a small range `[0,3]`, all values appear at least once (with and without skew)

### Outlier Tests

These use `generate()` with captured stdout to verify the outlier injection feature end-to-end.

* **Zero outliers**: `outliers=0` produces exactly `count` values with no regressions
* **Exact count**: `count + outliers` total lines are emitted
* **Zone bounds**: with `count=0` and `outliers=100`, every value falls within the expected edge zones
* **Validation**: negative count is rejected
* **Large run (ints)**: 10k normal + 50 outliers; edge zone count exceeds what uniform-only would produce
* **Float zones**: float outliers stay within the correct edge zones
* **Works with skew**: outliers + skew together produce the correct total count with all values in range

## Personal Project Disclosure

This program is my own original idea, conceived and developed entirely:

* On my own personal time, outside of work hours
* For my own personal benefit and use
* On my personally owned equipment
* Without using any employer resources, proprietary information, or trade secrets
* Without any connection to my employer's business, products, or services
* Independent of any duties or responsibilities of my employment

This project does not relate to my employer's actual or demonstrably
anticipated research, development, or business activities. No
confidential or proprietary information from any employer was used
in its creation.
