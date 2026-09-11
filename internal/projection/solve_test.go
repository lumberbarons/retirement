package projection

import (
	"math"
	"testing"
)

func TestSolveGross_NetsTargetWithinOneDollar(t *testing.T) {
	const (
		mandatory = 10000.0
		target    = 80000.0
		capacity  = 200000.0
	)
	net := func(withdrawal float64) float64 { return mandatory + withdrawal*0.7 } // 30% flat tax
	got := SolveGross(target, capacity, net)
	if diff := math.Abs(net(got) - target); diff > 1 {
		t.Fatalf("net at solved withdrawal = %v, want %v within $1 (off by %v)", net(got), target, diff)
	}
	if got <= 0 || got >= capacity {
		t.Fatalf("solved withdrawal = %v, want strictly inside (0, %v)", got, capacity)
	}
}

func TestSolveGross_ReturnsZeroWhenMandatoryIncomeMeetsTarget(t *testing.T) {
	got := SolveGross(50000, 100000, func(w float64) float64 { return 60000 + w })
	if got != 0 {
		t.Fatalf("withdrawal = %v, want 0 when mandatory income already meets the target", got)
	}
}

func TestSolveGross_ClampsAtCapacityWhenTargetUnreachable(t *testing.T) {
	got := SolveGross(80000, 15000, func(w float64) float64 { return w })
	if got != 15000 {
		t.Fatalf("withdrawal = %v, want capacity 15000 when the target cannot be reached", got)
	}
}

func TestSolveGross_ZeroCapacityNeedsNoWithdrawal(t *testing.T) {
	called := false
	net := func(w float64) float64 {
		called = true
		return w
	}
	if got := SolveGross(80000, 0, net); got != 0 {
		t.Fatalf("withdrawal = %v, want 0 for zero capacity", got)
	}
	if called {
		t.Fatal("zero capacity should short-circuit before evaluating net income")
	}
}

// TestSolveGross_ConvergesInOASClawbackZone models the OAS recovery zone: a
// 47% statutory rate plus the 15% recovery tax, an effective 62% marginal
// rate, above the 60% the governing spec calls out. The solve must still land
// on the target within $1.
func TestSolveGross_ConvergesInOASClawbackZone(t *testing.T) {
	const (
		mandatory = 100000.0
		threshold = 95000.0
		statutory = 0.47
		recovery  = 0.15
		target    = 130000.0
		capacity  = 500000.0
	)
	net := func(withdrawal float64) float64 {
		income := mandatory + withdrawal
		tax := statutory * income
		if income > threshold {
			tax += recovery * (income - threshold)
		}
		return income - tax
	}
	mtr := 1 - (net(mandatory+1000)-net(mandatory))/1000
	if mtr <= 0.60 {
		t.Fatalf("effective marginal rate = %v, want > 0.60 for this test to mean anything", mtr)
	}
	got := SolveGross(target, capacity, net)
	if diff := math.Abs(net(got) - target); diff > 1 {
		t.Fatalf("net at solved withdrawal = %v, want %v within $1 (off by %v)", net(got), target, diff)
	}
}

func TestRun_WithdrawalsAreProRataWithinATier(t *testing.T) {
	const yamlText = `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 40}
  - {name: Sam, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: Alex RRSP, type: rrsp, owner: Alex, balance: 300000}
  - {name: Sam RRSP, type: rrsp, owner: Sam, balance: 100000}
spending: {target_today_dollars: 20000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	first := results[0]
	// After the 5% return the registered tier is 315000/105000, so the
	// 20000 target splits 75/25 rather than draining config order.
	if got := accountYear(t, first, "Alex RRSP").Withdrawal; math.Abs(got-15000) > 0.01 {
		t.Fatalf("Alex withdrawal = %v, want 15000 (pro-rata)", got)
	}
	if got := accountYear(t, first, "Sam RRSP").Withdrawal; math.Abs(got-5000) > 0.01 {
		t.Fatalf("Sam withdrawal = %v, want 5000 (pro-rata)", got)
	}
}

func TestRun_RRSPAndRRIFShareTheRegisteredTier(t *testing.T) {
	const yamlText = `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 40}
  - {name: Sam, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: Alex RRSP, type: rrsp, owner: Alex, balance: 300000}
  - {name: Sam RRIF, type: rrif, owner: Sam, balance: 100000}
spending: {target_today_dollars: 20000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	first := results[0]
	// The RRSP alone could cover the target, so the old type-by-type order
	// would leave the RRIF untouched. The registered tier is shared now, so
	// the RRIF is drawn past its mandatory minimum.
	rrif := accountYear(t, first, "Sam RRIF")
	if rrif.Withdrawal <= first.MandatoryIncome {
		t.Fatalf("RRIF withdrawal = %v, want more than the mandatory minimum %v", rrif.Withdrawal, first.MandatoryIncome)
	}
	if rrsp := accountYear(t, first, "Alex RRSP"); rrsp.Withdrawal >= 20000 {
		t.Fatalf("RRSP withdrawal = %v, want less than the full 20000 target", rrsp.Withdrawal)
	}
}
