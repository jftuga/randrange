package main

import (
	"bufio"
	"encoding/binary"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// --- helpers ----------------------------------------------------------------

const tolerance = 1e-9

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

// deterministicSource builds a *randSource pre-loaded with known uint64 values.
// Each value is encoded little-endian into the buffer so Uint64() returns them
// in order. When the buffer is exhausted it wraps (crypto/rand.Read refills it,
// but for tests we pre-fill so the first len(vals) calls are deterministic).
func deterministicSource(vals ...uint64) *randSource {
	buf := make([]byte, len(vals)*8)
	for i, v := range vals {
		binary.LittleEndian.PutUint64(buf[i*8:], v)
	}
	return &randSource{buf: buf, pos: 0}
}

// uint64ForUnit returns the uint64 that, when masked to 53 bits and divided
// by (mask+1), produces a unit value closest to the target u in [0,1).
func uint64ForUnit(u float64) uint64 {
	const mask = (1 << 53) - 1
	return uint64(u * float64(mask+1))
}

// median returns the median of a sorted float64 slice.
func median(vals []float64) float64 {
	sort.Float64s(vals)
	n := len(vals)
	if n%2 == 0 {
		return (vals[n/2-1] + vals[n/2]) / 2
	}
	return vals[n/2]
}

// --- applySkew (deterministic) ----------------------------------------------

func TestApplySkew_Identity(t *testing.T) {
	// Strength 1.0 should return input unchanged for any skew direction.
	for _, skew := range []string{"low", "high"} {
		for _, u := range []float64{0.0, 0.25, 0.5, 0.75, 0.999} {
			got := applySkew(u, skew, 1.0)
			if !approxEqual(got, u, tolerance) {
				t.Errorf("applySkew(%v, %q, 1.0) = %v, want %v", u, skew, got, u)
			}
		}
	}
}

func TestApplySkew_NoSkew(t *testing.T) {
	// Empty string (no skew) returns input unchanged at any strength.
	for _, strength := range []float64{1.0, 2.0, 5.0} {
		for _, u := range []float64{0.0, 0.5, 0.99} {
			got := applySkew(u, "", strength)
			if !approxEqual(got, u, tolerance) {
				t.Errorf("applySkew(%v, \"\", %v) = %v, want %v", u, strength, got, u)
			}
		}
	}
}

func TestApplySkew_Boundaries(t *testing.T) {
	// u=0 and u=1 should map to 0 and 1 regardless of skew/strength.
	for _, skew := range []string{"low", "high", ""} {
		for _, strength := range []float64{1.0, 2.0, 5.0, 10.0} {
			got0 := applySkew(0.0, skew, strength)
			got1 := applySkew(1.0, skew, strength)
			if !approxEqual(got0, 0.0, tolerance) {
				t.Errorf("applySkew(0, %q, %v) = %v, want 0", skew, strength, got0)
			}
			if !approxEqual(got1, 1.0, tolerance) {
				t.Errorf("applySkew(1, %q, %v) = %v, want 1", skew, strength, got1)
			}
		}
	}
}

func TestApplySkew_KnownValues(t *testing.T) {
	tests := []struct {
		u        float64
		skew     string
		strength float64
		want     float64
	}{
		// low: u^s
		{0.5, "low", 2.0, 0.25},
		{0.5, "low", 3.0, 0.125},
		{0.25, "low", 2.0, 0.0625},
		// high: 1 - (1-u)^s
		{0.5, "high", 2.0, 0.75},
		{0.5, "high", 3.0, 0.875},
		{0.75, "high", 2.0, 0.9375},
	}
	for _, tc := range tests {
		got := applySkew(tc.u, tc.skew, tc.strength)
		if !approxEqual(got, tc.want, tolerance) {
			t.Errorf("applySkew(%v, %q, %v) = %v, want %v",
				tc.u, tc.skew, tc.strength, got, tc.want)
		}
	}
}

func TestApplySkew_Symmetry(t *testing.T) {
	// applySkew(x, "low", s) + applySkew(1-x, "high", s) should equal 1.
	for _, strength := range []float64{2.0, 3.0, 5.0} {
		for _, u := range []float64{0.1, 0.25, 0.5, 0.7, 0.9} {
			low := applySkew(u, "low", strength)
			high := applySkew(1-u, "high", strength)
			sum := low + high
			if !approxEqual(sum, 1.0, tolerance) {
				t.Errorf("symmetry broken: applySkew(%v,low,%v)=%v + applySkew(%v,high,%v)=%v = %v, want 1.0",
					u, strength, low, 1-u, strength, high, sum)
			}
		}
	}
}

func TestApplySkew_Monotonicity(t *testing.T) {
	// For fixed skew and strength, output must be monotonically increasing.
	inputs := []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	for _, skew := range []string{"low", "high"} {
		for _, strength := range []float64{2.0, 3.0, 5.0} {
			prev := -1.0
			for _, u := range inputs {
				got := applySkew(u, skew, strength)
				if got < prev {
					t.Errorf("monotonicity violated: applySkew(%v, %q, %v) = %v < prev %v",
						u, skew, strength, got, prev)
				}
				prev = got
			}
		}
	}
}

// --- randomInt (deterministic) ----------------------------------------------

func TestRandomInt_Bounds(t *testing.T) {
	// Use deterministic source with values that map to extremes.
	// u≈0 → should return min, u≈max_mask → should return max.
	rs := deterministicSource(uint64ForUnit(0.0), uint64ForUnit(0.9999999))
	v1, err := randomInt(rs, 10, 20, "", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if v1 != 10 {
		t.Errorf("unit≈0: got %d, want 10", v1)
	}

	v2, err := randomInt(rs, 10, 20, "", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if v2 < 10 || v2 > 20 {
		t.Errorf("unit≈1: got %d, want [10,20]", v2)
	}
}

func TestRandomInt_MinEqualsMax(t *testing.T) {
	rs := deterministicSource(uint64ForUnit(0.5))
	v, err := randomInt(rs, 42, 42, "", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if v != 42 {
		t.Errorf("min==max: got %d, want 42", v)
	}
}

func TestRandomInt_MinGreaterThanMax(t *testing.T) {
	rs := deterministicSource(0)
	_, err := randomInt(rs, 10, 5, "", 1.0)
	if err == nil {
		t.Error("expected error for min > max")
	}
}

func TestRandomInt_FullCoverage(t *testing.T) {
	// For a small range [0,3], confirm all values appear with enough samples.
	rs := newRandSource()
	seen := map[int]bool{}
	for i := 0; i < 1000; i++ {
		v, err := randomInt(rs, 0, 3, "", 1.0)
		if err != nil {
			t.Fatal(err)
		}
		if v < 0 || v > 3 {
			t.Fatalf("out of range: %d", v)
		}
		seen[v] = true
	}
	for i := 0; i <= 3; i++ {
		if !seen[i] {
			t.Errorf("value %d never appeared in 1000 samples", i)
		}
	}
}

func TestRandomInt_FullCoverageWithSkew(t *testing.T) {
	// Even with skew, all values in a small range should still appear.
	for _, skew := range []string{"low", "high"} {
		rs := newRandSource()
		seen := map[int]bool{}
		for i := 0; i < 10000; i++ {
			v, err := randomInt(rs, 0, 3, skew, 2.0)
			if err != nil {
				t.Fatal(err)
			}
			if v < 0 || v > 3 {
				t.Fatalf("out of range: %d", v)
			}
			seen[v] = true
		}
		for i := 0; i <= 3; i++ {
			if !seen[i] {
				t.Errorf("skew=%q: value %d never appeared in 10000 samples", skew, i)
			}
		}
	}
}

// --- randomFloat (deterministic) --------------------------------------------

func TestRandomFloat_Bounds(t *testing.T) {
	rs := deterministicSource(uint64ForUnit(0.0), uint64ForUnit(0.9999999))
	v1, err := randomFloat(rs, -5.0, 5.0, "", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if v1 < -5.0 || v1 >= 5.0 {
		t.Errorf("got %v, want [-5.0, 5.0)", v1)
	}

	v2, err := randomFloat(rs, -5.0, 5.0, "", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if v2 < -5.0 || v2 >= 5.0 {
		t.Errorf("got %v, want [-5.0, 5.0)", v2)
	}
}

func TestRandomFloat_Precision(t *testing.T) {
	// Every result should be rounded to 8 decimal places.
	rs := newRandSource()
	for i := 0; i < 10000; i++ {
		v, err := randomFloat(rs, 0.0, 100.0, "", 1.0)
		if err != nil {
			t.Fatal(err)
		}
		rounded := float64(int(v*1e8+0.5)) / 1e8
		if !approxEqual(v, rounded, 1e-12) {
			t.Errorf("value %v not rounded to 8 decimals (rounded=%v)", v, rounded)
		}
	}
}

func TestRandomFloat_MinGreaterThanMax(t *testing.T) {
	rs := deterministicSource(0)
	_, err := randomFloat(rs, 10.0, 5.0, "", 1.0)
	if err == nil {
		t.Error("expected error for min >= max")
	}
}

func TestRandomFloat_MinEqualsMax(t *testing.T) {
	rs := deterministicSource(0)
	_, err := randomFloat(rs, 5.0, 5.0, "", 1.0)
	if err == nil {
		t.Error("expected error for min >= max")
	}
}

// --- probabilistic distribution tests ---------------------------------------

const sampleSize = 500_000

func TestDistribution_UniformInts(t *testing.T) {
	// 500k ints in [0,9] bucketed into 10 bins should be roughly equal.
	// Expected: 50k per bin. Allow 10% deviation.
	rs := newRandSource()
	bins := make([]int, 10)
	for i := 0; i < sampleSize; i++ {
		v, err := randomInt(rs, 0, 9, "", 1.0)
		if err != nil {
			t.Fatal(err)
		}
		bins[v]++
	}
	expected := float64(sampleSize) / 10
	for i, count := range bins {
		ratio := float64(count) / expected
		if ratio < 0.90 || ratio > 1.10 {
			t.Errorf("bin %d: count=%d, expected≈%.0f, ratio=%.3f (outside 10%% tolerance)",
				i, count, expected, ratio)
		}
	}
}

func TestDistribution_UniformFloats(t *testing.T) {
	// 500k floats in [0,100) bucketed into 10 bins of width 10.
	rs := newRandSource()
	bins := make([]int, 10)
	for i := 0; i < sampleSize; i++ {
		v, err := randomFloat(rs, 0.0, 100.0, "", 1.0)
		if err != nil {
			t.Fatal(err)
		}
		bin := int(v / 10)
		if bin >= 10 {
			bin = 9
		}
		bins[bin]++
	}
	expected := float64(sampleSize) / 10
	for i, count := range bins {
		ratio := float64(count) / expected
		if ratio < 0.90 || ratio > 1.10 {
			t.Errorf("bin %d: count=%d, expected≈%.0f, ratio=%.3f", i, count, expected, ratio)
		}
	}
}

func TestDistribution_SkewLowMedian(t *testing.T) {
	// With skew=low, strength=2.0, the theoretical median of the unit
	// distribution is sqrt(0.5) ≈ 0.707, so the median should be at about
	// 0.707 * range. For [0,1000] ints the median should be well below 500.
	rs := newRandSource()
	vals := make([]float64, sampleSize)
	for i := range vals {
		v, err := randomInt(rs, 0, 1000, "low", 2.0)
		if err != nil {
			t.Fatal(err)
		}
		vals[i] = float64(v)
	}
	med := median(vals)
	// Theoretical median ≈ 0.5^2 * 1001 ≈ 250. Allow generous range.
	if med > 350 {
		t.Errorf("skew=low median=%v, expected well below 500", med)
	}
}

func TestDistribution_SkewHighMedian(t *testing.T) {
	// Mirror of SkewLowMedian: median should be well above 500.
	rs := newRandSource()
	vals := make([]float64, sampleSize)
	for i := range vals {
		v, err := randomInt(rs, 0, 1000, "high", 2.0)
		if err != nil {
			t.Fatal(err)
		}
		vals[i] = float64(v)
	}
	med := median(vals)
	if med < 650 {
		t.Errorf("skew=high median=%v, expected well above 500", med)
	}
}

func TestDistribution_HigherStrengthMoreExtreme(t *testing.T) {
	// Skew low with strength 5.0 should produce a lower median than strength 2.0.
	rs := newRandSource()
	vals2 := make([]float64, sampleSize)
	vals5 := make([]float64, sampleSize)
	for i := 0; i < sampleSize; i++ {
		v, err := randomInt(rs, 0, 1000, "low", 2.0)
		if err != nil {
			t.Fatal(err)
		}
		vals2[i] = float64(v)
	}
	for i := 0; i < sampleSize; i++ {
		v, err := randomInt(rs, 0, 1000, "low", 5.0)
		if err != nil {
			t.Fatal(err)
		}
		vals5[i] = float64(v)
	}
	med2 := median(vals2)
	med5 := median(vals5)
	if med5 >= med2 {
		t.Errorf("strength 5.0 median (%v) should be lower than strength 2.0 median (%v)", med5, med2)
	}
}

func TestDistribution_SkewLowHighSymmetry(t *testing.T) {
	// The histogram of skew=low should roughly mirror skew=high.
	// We compare: low's bottom-half count ≈ high's top-half count.
	rs := newRandSource()
	lowBelow := 0
	for i := 0; i < sampleSize; i++ {
		v, err := randomInt(rs, 0, 1000, "low", 3.0)
		if err != nil {
			t.Fatal(err)
		}
		if v <= 500 {
			lowBelow++
		}
	}
	highAbove := 0
	for i := 0; i < sampleSize; i++ {
		v, err := randomInt(rs, 0, 1000, "high", 3.0)
		if err != nil {
			t.Fatal(err)
		}
		if v >= 500 {
			highAbove++
		}
	}
	// These should be approximately equal.
	ratioLow := float64(lowBelow) / float64(sampleSize)
	ratioHigh := float64(highAbove) / float64(sampleSize)
	diff := math.Abs(ratioLow - ratioHigh)
	if diff > 0.02 {
		t.Errorf("asymmetry: low-below-500 ratio=%.4f, high-above-500 ratio=%.4f, diff=%.4f",
			ratioLow, ratioHigh, diff)
	}
}

func TestDistribution_SkewFloatMedian(t *testing.T) {
	// Verify skew works for floats too, not just ints.
	rs := newRandSource()
	vals := make([]float64, sampleSize)
	for i := range vals {
		v, err := randomFloat(rs, 0.0, 1000.0, "low", 3.0)
		if err != nil {
			t.Fatal(err)
		}
		vals[i] = v
	}
	med := median(vals)
	// Theoretical median for strength=3: 0.5^3 * 1000 = 125.
	if med > 200 {
		t.Errorf("skew=low float median=%v, expected well below 500", med)
	}
}

// --- randomInt bounds (probabilistic) ----------------------------------------

func TestRandomInt_BoundsLargeRun(t *testing.T) {
	// Verify no values escape [min, max] over a large run, with and without skew.
	configs := []struct {
		skew     string
		strength float64
	}{
		{"", 1.0},
		{"low", 2.0},
		{"high", 2.0},
		{"low", 5.0},
		{"high", 5.0},
	}
	for _, cfg := range configs {
		rs := newRandSource()
		for i := 0; i < 100_000; i++ {
			v, err := randomInt(rs, -50, 50, cfg.skew, cfg.strength)
			if err != nil {
				t.Fatal(err)
			}
			if v < -50 || v > 50 {
				t.Fatalf("skew=%q strength=%v: value %d outside [-50,50]",
					cfg.skew, cfg.strength, v)
			}
		}
	}
}

func TestRandomFloat_BoundsLargeRun(t *testing.T) {
	// Verify no values escape [min, max) over a large run, with and without skew.
	configs := []struct {
		skew     string
		strength float64
	}{
		{"", 1.0},
		{"low", 2.0},
		{"high", 2.0},
		{"low", 5.0},
		{"high", 5.0},
	}
	for _, cfg := range configs {
		rs := newRandSource()
		for i := 0; i < 100_000; i++ {
			v, err := randomFloat(rs, -50.0, 50.0, cfg.skew, cfg.strength)
			if err != nil {
				t.Fatal(err)
			}
			if v < -50.0 || v >= 50.0 {
				t.Fatalf("skew=%q strength=%v: value %v outside [-50.0, 50.0)",
					cfg.skew, cfg.strength, v)
			}
		}
	}
}

// --- outlier tests -----------------------------------------------------------

// captureGenerate runs generate() and captures its stdout output as a string.
func captureGenerate(t *testing.T, start, end, count int, floats bool, skew string, skewStrength float64, outliers, outlierPercent int) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	genErr := generate(start, end, count, floats, skew, skewStrength, outliers, outlierPercent)
	w.Close()
	os.Stdout = old

	var buf strings.Builder
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		buf.WriteString(scanner.Text())
		buf.WriteByte('\n')
	}
	r.Close()

	if genErr != nil {
		t.Fatalf("generate() error: %v", genErr)
	}
	return buf.String()
}

// parseInts parses newline-separated integers from output.
func parseInts(t *testing.T, output string) []int {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	vals := make([]int, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("parseInts: %q: %v", line, err)
		}
		vals = append(vals, v)
	}
	return vals
}

// parseFloats parses newline-separated floats from output.
func parseFloats(t *testing.T, output string) []float64 {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	vals := make([]float64, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := strconv.ParseFloat(line, 64)
		if err != nil {
			t.Fatalf("parseFloats: %q: %v", line, err)
		}
		vals = append(vals, v)
	}
	return vals
}

func TestOutlier_ZeroOutliers(t *testing.T) {
	// outliers=0 should produce exactly count values, same as before.
	out := captureGenerate(t, 0, 1000, 20, false, "", 1.0, 0, 10)
	vals := parseInts(t, out)
	if len(vals) != 20 {
		t.Errorf("expected 20 values, got %d", len(vals))
	}
	for i, v := range vals {
		if v < 0 || v > 1000 {
			t.Errorf("value[%d]=%d out of [0,1000]", i, v)
		}
	}
}

func TestOutlier_ExactCount(t *testing.T) {
	// With outliers=5, count=20, we should get exactly 25 output lines.
	out := captureGenerate(t, 0, 1000, 20, false, "", 1.0, 5, 10)
	vals := parseInts(t, out)
	if len(vals) != 25 {
		t.Errorf("expected 25 values (20+5), got %d", len(vals))
	}
}

func TestOutlier_ZoneBounds(t *testing.T) {
	// With range [0,1000] and outlier-percent=10, edge zones are [0,100] and [900,1000].
	// Run many times and verify all outlier-zone values are within bounds.
	// We generate 0 normal + 100 outliers to get only outlier values.
	out := captureGenerate(t, 0, 1000, 0, false, "", 1.0, 100, 10)
	vals := parseInts(t, out)
	if len(vals) != 100 {
		t.Fatalf("expected 100 outlier values, got %d", len(vals))
	}
	for i, v := range vals {
		inLow := v >= 0 && v <= 100
		inHigh := v >= 900 && v <= 1000
		if !inLow && !inHigh {
			t.Errorf("outlier[%d]=%d not in [0,100] or [900,1000]", i, v)
		}
	}
}

func TestOutlier_Validation(t *testing.T) {
	// Test that generate rejects negative count (proxy for validation).
	err := generate(0, 100, -1, false, "", 1.0, 0, 10)
	if err == nil {
		t.Error("expected error for negative count")
	}
}

func TestOutlier_LargeRunIntCount(t *testing.T) {
	// Generate 10k normal + 50 outliers in [0, 1000], percent=10.
	// Edge zones: [0,100] and [900,1000]. Total output: 10050 lines.
	out := captureGenerate(t, 0, 1000, 10_000, false, "", 1.0, 50, 10)
	vals := parseInts(t, out)
	if len(vals) != 10_050 {
		t.Fatalf("expected 10050 values, got %d", len(vals))
	}
	edgeCount := 0
	for _, v := range vals {
		if v <= 100 || v >= 900 {
			edgeCount++
		}
	}
	// With uniform distribution over [0,1000], about 20.1% fall in edges.
	// 10k * 0.201 ≈ 2010, plus 50 outliers. Edge count should exceed
	// the expected uniform edge count.
	uniformEdgeExpected := float64(10_000) * 201.0 / 1001.0
	if float64(edgeCount) < uniformEdgeExpected {
		t.Errorf("edge count %d lower than uniform expectation %.0f; outliers may not be working",
			edgeCount, uniformEdgeExpected)
	}
}

func TestOutlier_FloatZones(t *testing.T) {
	// Generate 0 normal + 100 outliers as floats in [0, 1000), percent=10.
	// Edge zones: [0, 100) and [900, 1000).
	out := captureGenerate(t, 0, 1000, 0, true, "", 1.0, 100, 10)
	vals := parseFloats(t, out)
	if len(vals) != 100 {
		t.Fatalf("expected 100 float outlier values, got %d", len(vals))
	}
	for i, v := range vals {
		inLow := v >= 0.0 && v < 100.0
		inHigh := v >= 900.0 && v < 1000.0
		if !inLow && !inHigh {
			t.Errorf("float outlier[%d]=%v not in [0,100) or [900,1000)", i, v)
		}
	}
}

func TestOutlier_WorksWithSkew(t *testing.T) {
	// Outliers + skew together: total count should be correct and
	// outlier values should still be in edge zones.
	out := captureGenerate(t, 0, 1000, 100, false, "low", 2.0, 10, 10)
	vals := parseInts(t, out)
	if len(vals) != 110 {
		t.Fatalf("expected 110 values (100+10), got %d", len(vals))
	}
	// All values should be in [0, 1000].
	for i, v := range vals {
		if v < 0 || v > 1000 {
			t.Errorf("value[%d]=%d out of [0,1000]", i, v)
		}
	}
}
