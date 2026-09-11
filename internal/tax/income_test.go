package tax

import "testing"

func TestIncome_EligiblePensionPerType(t *testing.T) {
	in := Income{
		RRSPWithdrawals: 1000,
		RRIFWithdrawals: 2000,
		DBPPension:      3000,
		CPP:             4000,
		OAS:             5000,
	}
	if got := in.EligiblePension(50); got != 3000 {
		t.Fatalf("eligible pension at 50 = %v, want DB only (3000): RRSP, CPP, and OAS never qualify", got)
	}
	if got := in.EligiblePension(64); got != 3000 {
		t.Fatalf("eligible pension at 64 = %v, want DB only (3000): RRIF qualifies at 65+", got)
	}
	if got := in.EligiblePension(65); got != 5000 {
		t.Fatalf("eligible pension at 65 = %v, want DB + RRIF (5000)", got)
	}
	if got := in.EligiblePension(70); got != 5000 {
		t.Fatalf("eligible pension at 70 = %v, want DB + RRIF (5000)", got)
	}
}

func TestIncome_GrossedUpEligibleDividends(t *testing.T) {
	in := Income{EligibleDividends: 10000}
	rates, _, err := constantsIncome(t)
	if err != nil {
		t.Fatalf("constants.Income.For: %v", err)
	}
	taxable, fedDTC, onDTC := in.grossedUp(rates)
	almostEqual(t, taxable, 13800)
	almostEqual(t, fedDTC, 13800*0.150198)
	almostEqual(t, onDTC, 1380)
}

func TestIncome_GrossedUpNonEligibleDividends(t *testing.T) {
	in := Income{NonEligibleDividends: 10000}
	rates, _, err := constantsIncome(t)
	if err != nil {
		t.Fatalf("constants.Income.For: %v", err)
	}
	taxable, fedDTC, onDTC := in.grossedUp(rates)
	almostEqual(t, taxable, 11500)
	almostEqual(t, fedDTC, 11500*0.090301)
	almostEqual(t, onDTC, 11500*0.029863)
}

func TestIncome_CapitalGainsIncludeAtFiftyPercent(t *testing.T) {
	in := Income{CapitalGains: 10000}
	rates, _, err := constantsIncome(t)
	if err != nil {
		t.Fatalf("constants.Income.For: %v", err)
	}
	taxable, fedDTC, onDTC := in.grossedUp(rates)
	almostEqual(t, taxable, 5000)
	if fedDTC != 0 || onDTC != 0 {
		t.Fatalf("capital gains produced DTCs %v/%v, want none", fedDTC, onDTC)
	}
}

func TestIncome_AllComponentsTaxable(t *testing.T) {
	in := Income{
		Employment:           10000,
		RRSPWithdrawals:      20000,
		RRIFWithdrawals:      30000,
		DBPPension:           40000,
		CPP:                  5000,
		OAS:                  6000,
		Interest:             7000,
		EligibleDividends:    1000,
		NonEligibleDividends: 2000,
		CapitalGains:         3000,
	}
	rates, _, err := constantsIncome(t)
	if err != nil {
		t.Fatalf("constants.Income.For: %v", err)
	}
	want := 10000.0 + 20000 + 30000 + 40000 + 5000 + 6000 + 7000 +
		1000*(1+rates.GrossUpEligible) + 2000*(1+rates.GrossUpNonEligible) + 1500
	taxable, _, _ := in.grossedUp(rates)
	almostEqual(t, taxable, want)
}
