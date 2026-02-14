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

| Flag     | Default | Meaning |
|----------|---------|---------|
| `-start`  | 0       | Inclusive lower bound |
| `-end`    | 100     | Inclusive upper bound |
| `-count`  | 5       | How many numbers to emit |
| `-floats` | false   | Generate floats (8-decimal precision) instead of integers |

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

## Progress Status

On macOS and BSD systems, pressing **Ctrl-T** while `randrange` is running sends a `SIGINFO` signal, which causes it to print the current progress to stderr:

```
progress: 500000 / 1000000 (50.0%)
```

This is useful when generating a large quantity of numbers (e.g., `-count 1000000`) and you want to check how far along the run is. On other platforms this signal is not available and Ctrl-T has no effect.

## Notes

* Uses `crypto/rand` for cryptographic strength.
* Floats are rounded to 8 decimal places.
* No external dependencies; plain `go run` or `go build` is enough.

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
