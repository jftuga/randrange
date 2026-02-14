package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
)

const pgmName = "randrange"
const pgmVersion = "0.3.1"
const pgmUrl = "https://github.com/jftuga/randrange"
const pgmDisclaimer = "DISCLAIMER: This program is vibe-coded. Use at your own risk."

// randSource batches reads from crypto/rand to reduce syscall overhead.
type randSource struct {
	buf []byte
	pos int
}

const randBufSize = 4096 // 512 uint64s per syscall

func newRandSource() *randSource {
	return &randSource{buf: make([]byte, randBufSize), pos: randBufSize}
}

func (r *randSource) Uint64() (uint64, error) {
	if r.pos+8 > len(r.buf) {
		if _, err := rand.Read(r.buf); err != nil {
			return 0, err
		}
		r.pos = 0
	}
	v := binary.LittleEndian.Uint64(r.buf[r.pos : r.pos+8])
	r.pos += 8
	return v, nil
}

// applySkew transforms a uniform [0,1) value using a power-law curve.
// skew "low" clusters values toward 0 (start), "high" toward 1 (end).
// strength 1.0 = uniform (no skew); higher values = more aggressive skew.
func applySkew(u float64, skew string, strength float64) float64 {
	switch skew {
	case "low":
		return math.Pow(u, strength)
	case "high":
		return 1 - math.Pow(1-u, strength)
	default:
		return u
	}
}

func randomInt(rs *randSource, min, max int, skew string, strength float64) (int, error) {
	if min > max {
		return 0, fmt.Errorf("min (%d) > max (%d)", min, max)
	}
	const mask = (1 << 53) - 1
	n, err := rs.Uint64()
	if err != nil {
		return 0, err
	}
	unit := float64(n&mask) / float64(mask+1) // [0,1)
	unit = applySkew(unit, skew, strength)
	span := float64(max - min + 1)
	v := int(unit*span) + min
	if v > max {
		v = max
	}
	return v, nil
}

// randomFloat returns a float64 in [min, max) rounded to 8 decimal places.
func randomFloat(rs *randSource, min, max float64, skew string, strength float64) (float64, error) {
	if min >= max {
		return 0, fmt.Errorf("min (%f) >= max (%f)", min, max)
	}
	// 53 bits of precision in the mantissa
	const mask = (1 << 53) - 1
	n, err := rs.Uint64()
	if err != nil {
		return 0, err
	}
	unit := float64(n&mask) / float64(mask+1) // [0,1)
	unit = applySkew(unit, skew, strength)
	val := min + unit*(max-min)
	result := float64(int(val*1e8+0.5)) / 1e8
	if result >= max {
		result = math.Nextafter(max, min) // clamp to just below max
	}
	return result, nil
}

// generate prints count random numbers in [start, end], one per line.
func generate(start, end int, count int, floats bool, skew string, skewStrength float64) error {
	if count < 0 {
		return fmt.Errorf("negative count: %d", count)
	}

	sigCh := make(chan os.Signal, 1)
	registerSIGINFO(sigCh)
	defer signal.Stop(sigCh)

	rs := newRandSource()
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	for i := 0; i < count; i++ {
		// Check for SIGINFO (ctrl-t) without blocking.
		select {
		case <-sigCh:
			pct := float64(i) / float64(count) * 100
			fmt.Fprintf(os.Stderr, "progress: %d / %d (%.1f%%)\n", i, count, pct)
		default:
		}

		if floats {
			v, err := randomFloat(rs, float64(start), float64(end), skew, skewStrength)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "%.8f\n", v)
		} else {
			v, err := randomInt(rs, start, end, skew, skewStrength)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, v)
		}
	}
	return nil
}

func main() {
	var (
		start        = flag.Int("start", 0, "inclusive lower bound")
		end          = flag.Int("end", 100, "inclusive upper bound")
		count        = flag.Int("count", 5, "how many numbers to emit")
		floats       = flag.Bool("floats", false, "generate floats instead of integers")
		skew         = flag.String("skew", "", "skew distribution: 'low' (toward start) or 'high' (toward end)")
		skewStrength = flag.Float64("skew-strength", 2.0, "skew intensity: 1.0=uniform, higher=more skewed")
		version      = flag.Bool("version", false, "display version and exit")
	)
	flag.Parse()

	if *version {
		fmt.Printf("%s v%s\n%s\n\n%s\n", pgmName, pgmVersion, pgmUrl, pgmDisclaimer)
		os.Exit(0)
	}

	if *skew != "" && *skew != "low" && *skew != "high" {
		fmt.Fprintln(os.Stderr, "error: -skew must be 'low' or 'high'")
		os.Exit(1)
	}
	if *skewStrength < 1.0 {
		fmt.Fprintln(os.Stderr, "error: -skew-strength must be >= 1.0")
		os.Exit(1)
	}
	skewStrengthSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "skew-strength" {
			skewStrengthSet = true
		}
	})
	if skewStrengthSet && *skew == "" {
		fmt.Fprintln(os.Stderr, "error: -skew-strength requires -skew to be set")
		os.Exit(1)
	}

	if err := generate(*start, *end, *count, *floats, *skew, *skewStrength); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
