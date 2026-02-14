# randrange

A tiny, zero-dependency Go utility that prints cryptographically-strong random numbers to stdout.

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

## Notes

* Uses `crypto/rand` for cryptographic strength.
* Floats are rounded to 8 decimal places.
* No external dependencies; plain `go run` or `go build` is enough.
