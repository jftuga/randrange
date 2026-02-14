package main

import (
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/signal"
)

// randomInt unchanged
func randomInt(min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("min (%d) > max (%d)", min, max)
	}
	span := uint64(max - min + 1)
	n, err := randUint64()
	if err != nil {
		return 0, err
	}
	return int(n%span) + min, nil
}

// randUint64 returns a cryptographically-strong pseudo-random uint64.
func randUint64() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

// randomFloat returns a uniform float64 in [min, max) rounded to 4 decimal places.
func randomFloat(min, max float64) (float64, error) {
	if min >= max {
		return 0, fmt.Errorf("min (%f) >= max (%f)", min, max)
	}
	// 53 bits of precision in the mantissa
	const mask = (1 << 53) - 1
	n, err := randUint64()
	if err != nil {
		return 0, err
	}
	unit := float64(n&mask) / float64(mask+1) // [0,1)
	val := min + unit*(max-min)
	return float64(int(val*1e4+0.5)) / 1e4, nil
}

// generate prints count random numbers in [start, end], one per line.
func generate(start, end int, count int, floats bool) error {
	if count < 0 {
		return fmt.Errorf("negative count: %d", count)
	}

	sigCh := make(chan os.Signal, 1)
	registerSIGINFO(sigCh)
	defer signal.Stop(sigCh)

	for i := 0; i < count; i++ {
		// Check for SIGINFO (ctrl-t) without blocking.
		select {
		case <-sigCh:
			pct := float64(i) / float64(count) * 100
			fmt.Fprintf(os.Stderr, "progress: %d / %d (%.1f%%)\n", i, count, pct)
		default:
		}

		if floats {
			v, err := randomFloat(float64(start), float64(end))
			if err != nil {
				return err
			}
			fmt.Printf("%.4f\n", v)
		} else {
			v, err := randomInt(start, end)
			if err != nil {
				return err
			}
			fmt.Println(v)
		}
	}
	return nil
}

func main() {
	var (
		start  = flag.Int("start", 0, "inclusive lower bound")
		end    = flag.Int("end", 100, "inclusive upper bound")
		count  = flag.Int("count", 5, "how many numbers to emit")
		floats = flag.Bool("floats", false, "generate floats instead of integers")
	)
	flag.Parse()

	if err := generate(*start, *end, *count, *floats); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
