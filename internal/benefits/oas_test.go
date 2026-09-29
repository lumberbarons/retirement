package benefits

import (
	"math"
	"testing"
)

// TestOASFactor_DeferralTo70 covers the Done-when item that OAS defers up to
// +36% at 70.
func TestOASFactor_DeferralTo70(t *testing.T) {
	cases := []struct {
		startAge int
		want     float64
	}{
		{65, 1.000},
		{68, 1 + 0.006*12*3},
		{70, 1.360},
	}
	for _, c := range cases {
		got, err := OASFactor(c.startAge, 2026, forward)
		if err != nil {
			t.Fatalf("OASFactor(%d): %v", c.startAge, err)
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("OASFactor(%d) = %v, want %v", c.startAge, got, c.want)
		}
	}
	for _, bad := range []int{64, 71} {
		if _, err := OASFactor(bad, 2026, forward); err == nil {
			t.Fatalf("OASFactor(%d) should error outside 65-70, got nil", bad)
		}
	}
}

// TestOASAnnual_SumsTheFourQuarterlyRates covers the Done-when item that the
// 2026 pension uses all four quarterly periods, so the year is not twelve
// times the January amount.
func TestOASAnnual_SumsTheFourQuarterlyRates(t *testing.T) {
	got, err := OASAnnual(65, 1961, 2026, forward)
	if err != nil {
		t.Fatalf("OASAnnual: %v", err)
	}
	want := 3*742.31 + 3*743.05 + 3*751.97 + 3*762.50
	withinDollar(t, got, want)
	if january := 12 * 742.31; math.Abs(got-january) <= 1 {
		t.Fatalf("2026 OAS = %v, want the four-quarter total %v, not 12 x January %v", got, want, january)
	}
}

// TestOASAnnual_FutureYearsApplyOneAnnualCPI covers the Done-when item that
// later years grow the four-quarter total by the documented annual CPI
// convention: one factor per year, never re-applied quarterly within the year.
func TestOASAnnual_FutureYearsApplyOneAnnualCPI(t *testing.T) {
	annual := func(year int) float64 {
		t.Helper()
		got, err := OASAnnual(65, 1962, year, forward)
		if err != nil {
			t.Fatalf("OASAnnual(%d): %v", year, err)
		}
		return got
	}
	base := 3*742.31 + 3*743.05 + 3*751.97 + 3*762.50

	withinDollar(t, annual(2027), base*1.021)
	withinDollar(t, annual(2036), base*math.Pow(1.021, 10))
	if doubleIndexed := base * math.Pow(1.021, 20); math.Abs(annual(2036)-doubleIndexed) <= 1 {
		t.Fatalf("2036 OAS = %v, want one CPI factor per year, got a double-indexed %v", annual(2036), doubleIndexed)
	}
}

// TestOASAnnual_StartYearAndAge75Treatment covers the Done-when item that both
// transitions are explicit: nothing before the start year, the full annual
// amount in the start year (no pro-rating), the 75+ quarterly rates from the
// year the spouse turns 75, and the deferral factor applied on top.
func TestOASAnnual_StartYearAndAge75Treatment(t *testing.T) {
	annual := func(startAge, birthYear, year int) float64 {
		t.Helper()
		got, err := OASAnnual(startAge, birthYear, year, forward)
		if err != nil {
			t.Fatalf("OASAnnual(%d, %d, %d): %v", startAge, birthYear, year, err)
		}
		return got
	}
	base65 := 3*742.31 + 3*743.05 + 3*751.97 + 3*762.50
	base75 := 3*816.54 + 3*817.36 + 3*827.17 + 3*838.75

	if got := annual(70, 1961, 2026); got != 0 {
		t.Fatalf("OAS before the start year = %v, want 0", got)
	}
	// The start year pays the full annual amount.
	withinDollar(t, annual(65, 1961, 2026), base65)
	// Age is taken at December 31: the 75+ rates begin in the year of the
	// spouse's 75th birthday.
	withinDollar(t, annual(65, 1951, 2026), base75)
	withinDollar(t, annual(65, 1952, 2026), base65)
	// Deferral applies on top of the applicable quarterly total.
	withinDollar(t, annual(70, 1956, 2026), base65*1.36)
	withinDollar(t, annual(70, 1951, 2026), base75*1.36)

	if _, err := OASAnnual(64, 1961, 2026, forward); err == nil {
		t.Fatal("OASAnnual with start age 64 should error")
	}
}

// TestOASRecovery_FullClawbackUsesAnnualPension covers the same-year model's
// full-recovery point. Because the model pays the four-quarter annual total,
// it reaches zero at threshold + annual pension / 15%, not at a published
// July-to-June recovery-range ceiling based on a different payment period.
func TestOASRecovery_FullClawbackUsesAnnualPension(t *testing.T) {
	recovery := func(oas, netIncome float64, year int) float64 {
		t.Helper()
		got, err := OASRecovery(oas, netIncome, year, forward)
		if err != nil {
			t.Fatalf("OASRecovery: %v", err)
		}
		return got
	}

	oas65, err := OASAnnual(65, 1961, 2026, forward)
	if err != nil {
		t.Fatalf("OASAnnual 65-74: %v", err)
	}
	oas75, err := OASAnnual(65, 1951, 2026, forward)
	if err != nil {
		t.Fatalf("OASAnnual 75+: %v", err)
	}

	if got := recovery(oas65, 95323, 2026); got != 0 {
		t.Fatalf("recovery at the threshold = %v, want 0", got)
	}
	withinDollar(t, recovery(oas65, 95323+10000, 2026), 0.15*10000)
	withinDollar(t, oas65-recovery(oas65, 155319.60, 2026), 0)
	withinDollar(t, oas75-recovery(oas75, 161319.40, 2026), 0)
	// The recovery can never exceed the pension itself.
	withinDollar(t, recovery(oas65, 1_000_000, 2026), oas65)
	// A later year's threshold is indexed, so the same net income recovers
	// nothing once the threshold has caught up with it.
	withinDollar(t, recovery(oas65, 95323, 2036), 0)
}
