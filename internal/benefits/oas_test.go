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

// TestOASAnnual_BaseRatesAgeBoostAndDeferral covers the Done-when item's
// published amounts: $742.31/mo at 65-74, the permanent +10% ($816.54/mo) at
// 75+, the deferral factor on top, and CPI indexation.
func TestOASAnnual_BaseRatesAgeBoostAndDeferral(t *testing.T) {
	annual := func(startAge, birthYear, year int) float64 {
		t.Helper()
		got, err := OASAnnual(startAge, birthYear, year, forward)
		if err != nil {
			t.Fatalf("OASAnnual: %v", err)
		}
		return got
	}

	if got := annual(70, 1961, 2026); got != 0 {
		t.Fatalf("OAS before the start year = %v, want 0", got)
	}
	withinDollar(t, annual(65, 1961, 2026), 12*742.31)
	withinDollar(t, annual(65, 1961, 2036), 12*816.54*math.Pow(1.021, 10))
	withinDollar(t, annual(70, 1956, 2026), 12*742.31*1.36)
	withinDollar(t, annual(70, 1956, 2031), 12*816.54*1.36*math.Pow(1.021, 5))

	if _, err := OASAnnual(64, 1961, 2026, forward); err == nil {
		t.Fatal("OASAnnual with start age 64 should error")
	}
}

// TestOASRecovery_FullClawbackAtESDCCeilings covers the Done-when item that
// the 15% recovery tax reaches zero net OAS at the ESDC full-clawback ceilings
// ($154,708 at 65-74, $160,647 at 75+).
func TestOASRecovery_FullClawbackAtESDCCeilings(t *testing.T) {
	recovery := func(oas, netIncome float64, year int) float64 {
		t.Helper()
		got, err := OASRecovery(oas, netIncome, year, forward)
		if err != nil {
			t.Fatalf("OASRecovery: %v", err)
		}
		return got
	}

	oas65 := 12 * 742.31
	oas75 := 12 * 816.54

	if got := recovery(oas65, 95323, 2026); got != 0 {
		t.Fatalf("recovery at the threshold = %v, want 0", got)
	}
	withinDollar(t, recovery(oas65, 95323+10000, 2026), 0.15*10000)
	withinDollar(t, oas65-recovery(oas65, 154708, 2026), 0)
	withinDollar(t, oas75-recovery(oas75, 160647, 2026), 0)
	// The recovery can never exceed the pension itself.
	withinDollar(t, recovery(oas65, 1_000_000, 2026), oas65)
	// A later year's threshold is indexed, so the same net income recovers
	// nothing once the threshold has caught up with it.
	withinDollar(t, recovery(oas65, 95323, 2036), 0)
}
