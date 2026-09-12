package benefits

import (
	"math"
	"testing"

	"github.com/lumberbarons/retirement/internal/constants"
)

var forward = constants.Forward{CPI: 0.021, Wage: 0.031}

// withinDollar asserts the ±$1 tolerance the governing spec's validation
// cases use for published benefit figures.
func withinDollar(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1 {
		t.Fatalf("got %v, want %v within $1", got, want)
	}
}

// TestCPPFactor_StartAgeScaling covers the Done-when item that CPP scales by
// start age: 60 → 0.640x, 70 → 1.420x the stated age-65 entitlement.
func TestCPPFactor_StartAgeScaling(t *testing.T) {
	cases := []struct {
		startAge int
		want     float64
	}{
		{60, 0.640},
		{62, 1 - 0.006*12*3},
		{65, 1.000},
		{68, 1 + 0.007*12*3},
		{70, 1.420},
	}
	for _, c := range cases {
		got, err := CPPFactor(c.startAge, 2026, forward)
		if err != nil {
			t.Fatalf("CPPFactor(%d): %v", c.startAge, err)
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("CPPFactor(%d) = %v, want %v", c.startAge, got, c.want)
		}
	}
	for _, bad := range []int{59, 71} {
		if _, err := CPPFactor(bad, 2026, forward); err == nil {
			t.Fatalf("CPPFactor(%d) should error outside 60-70, got nil", bad)
		}
	}
}

// TestCPPAnnual_ScalesEntitlementAndIndexesInPay covers the Done-when item
// that the stated entitlement is scaled by the start-age factor and indexes
// in pay: nothing before the start year, and CPI applied from the base year
// through the projection year.
func TestCPPAnnual_ScalesEntitlementAndIndexesInPay(t *testing.T) {
	annual := func(monthly float64, startAge, birthYear, year int) float64 {
		t.Helper()
		got, err := CPPAnnual(monthly, startAge, birthYear, year, 2026, forward)
		if err != nil {
			t.Fatalf("CPPAnnual: %v", err)
		}
		return got
	}

	if got := annual(1000, 65, 1966, 2026); got != 0 {
		t.Fatalf("CPP before the start year = %v, want 0", got)
	}
	withinDollar(t, annual(1000, 60, 1966, 2026), 12*1000*0.640)
	withinDollar(t, annual(1000, 70, 1956, 2026), 12*1000*1.420)
	withinDollar(t, annual(1000, 65, 1966, 2031), 12*1000*math.Pow(1.021, 5))
	withinDollar(t, annual(1000, 65, 1966, 2041), 12*1000*math.Pow(1.021, 15))

	if _, err := CPPAnnual(1000, 59, 1966, 2026, 2026, forward); err == nil {
		t.Fatal("CPPAnnual with start age 59 should error")
	}
}

// TestCPPSurvivorAnnual_CombinedCap covers the Done-when item that the CPP
// survivor benefit is computed under the combined cap: near-max earners get
// little or no top-up, a low-entitlement survivor gets the computed benefit,
// and the survivor's under-65 formula carries the flat component.
func TestCPPSurvivorAnnual_CombinedCap(t *testing.T) {
	survivor := func(deceased, own float64, survivorBirthYear, year int) float64 {
		t.Helper()
		got, err := CPPSurvivorAnnual(deceased, own, survivorBirthYear, year, 2026, forward)
		if err != nil {
			t.Fatalf("CPPSurvivorAnnual: %v", err)
		}
		return got
	}

	// Both near the maximum: the combined cap leaves only cap - own as top-up.
	withinDollar(t, survivor(1507.65, 1507.65, 1956, 2026), 12*(1531.56-1507.65))
	// A survivor with no pension and a modest deceased pension receives the
	// full computed survivor benefit, 60% at 65+.
	withinDollar(t, survivor(1000, 0, 1956, 2026), 12*0.60*1000)
	// Under 65 the benefit is the flat amount plus 37.5% of the deceased's.
	flat := 803.54 - 0.375*1507.65
	withinDollar(t, survivor(1000, 0, 1964, 2026), 12*(flat+0.375*1000))
	// The survivor's own deferral is not part of the cap comparison, so a
	// near-max survivor still receives only the small top-up.
	withinDollar(t, survivor(1507.65, 1507.65, 1962, 2026), 12*(1531.56-1507.65))
	// The benefit indexes in pay to the projection year.
	withinDollar(t, survivor(1000, 0, 1966, 2036), 12*0.60*1000*math.Pow(1.021, 10))
}

// TestShareCPP_DefaultOffAndShiftsIncome covers the Done-when item that a
// user-set sharing election is honoured (default off): the combined total is
// unchanged and the election shifts income to the lower earner.
func TestShareCPP_DefaultOffAndShiftsIncome(t *testing.T) {
	first, second, err := ShareCPP(1000, 400, 0)
	if err != nil {
		t.Fatalf("ShareCPP(default): %v", err)
	}
	if first != 1000 || second != 400 {
		t.Fatalf("default sharing moved pensions to %v/%v, want 1000/400", first, second)
	}

	first, second, err = ShareCPP(1000, 400, 1)
	if err != nil {
		t.Fatalf("ShareCPP(full): %v", err)
	}
	withinDollar(t, first, 700)
	withinDollar(t, second, 700)

	first, second, err = ShareCPP(1000, 400, 0.5)
	if err != nil {
		t.Fatalf("ShareCPP(half): %v", err)
	}
	withinDollar(t, first, 850)
	withinDollar(t, second, 550)
	if math.Abs((first+second)-1400) > 1 {
		t.Fatalf("sharing changed the combined total to %v, want 1400", first+second)
	}

	for _, bad := range []float64{-0.1, 1.1} {
		if _, _, err := ShareCPP(1000, 400, bad); err == nil {
			t.Fatalf("ShareCPP fraction %v should error", bad)
		}
	}
}
