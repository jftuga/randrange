package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/signal"
)

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

func randomInt(rs *randSource, min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("min (%d) > max (%d)", min, max)
	}
	span := uint64(max - min + 1)
	n, err := rs.Uint64()
	if err != nil {
		return 0, err
	}
	return int(n%span) + min, nil
}

// randomFloat returns a uniform float64 in [min, max) rounded to 4 decimal places.
func randomFloat(rs *randSource, min, max float64) (float64, error) {
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
			v, err := randomFloat(rs, float64(start), float64(end))
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "%.4f\n", v)
		} else {
			v, err := randomInt(rs, start, end)
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
