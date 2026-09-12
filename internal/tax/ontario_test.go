package tax

import (
	"testing"

	"github.com/lumberbarons/retirement/internal/constants"
)

func ohpBands(t *testing.T) []constants.OHPBand {
	t.Helper()
	row, _, err := constants.Tax.For(2026)
	if err != nil {
		t.Fatalf("constants.Tax.For(2026): %v", err)
	}
	return row.ONHealthPremium
}

func TestHealthPremium_AllPlateaus(t *testing.T) {
	bands := ohpBands(t)
	cases := []struct {
		income float64
		want   float64
	}{
		{0, 0},
		{20000, 0},
		{30000, 300},
		{36000, 300},
		{37000, 360},
		{48000, 450},
		{48300, 525},
		{72000, 600},
		{72300, 675},
		{200000, 750},
		{200600, 900},
		{250000, 900},
	}
	for _, c := range cases {
		almostEqual(t, HealthPremium(c.income, bands), c.want)
	}
}

func TestSurtax_Tiers(t *testing.T) {
	almostEqual(t, Surtax(5818, 5818, 7446), 0)
	almostEqual(t, Surtax(7446, 5818, 7446), 0.20*(7446-5818))
	almostEqual(t, Surtax(8000, 5818, 7446), 0.20*(8000-5818)+0.36*(8000-7446))
}

func TestCompute_EndToEndHandComputed(t *testing.T) {
	res, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", Age: 70, Income: Income{RRIFWithdrawals: 100000}},
		{Name: "B", Age: 70},
	}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	a := res.Spouses[0]
	almostEqual(t, a.TaxableIncome, 100000)
	almostEqual(t, a.FederalTaxBeforeCredits, 16696.005)
	almostEqual(t, a.FederalTax, 10356.133)
	almostEqual(t, a.OntarioBasicTax, 6940.469)
	almostEqual(t, a.OntarioBasicTaxAfterCredits, 5316.591)
	almostEqual(t, a.OntarioSurtax, 0)
	almostEqual(t, a.OntarioHealthPremium, 750)
	almostEqual(t, a.OntarioTax, 6066.591)
	almostEqual(t, res.Total, 16422.724)
	// B has no income: B's unused age credit transfers to A.
	almostEqual(t, res.Spouses[1].CreditTransferredOut, 1289.12+320.271)
	almostEqual(t, a.CreditTransferredIn, res.Spouses[1].CreditTransferredOut)
}

func TestCompute_OntarioSurtaxUsesTaxAfterCredits(t *testing.T) {
	res, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", Age: 70, Income: Income{RRIFWithdrawals: 100000}},
		{Name: "B", Age: 70, Income: Income{RRIFWithdrawals: 100000}},
	}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	for _, sr := range res.Spouses {
		if sr.OntarioBasicTax <= 5818 {
			t.Fatalf("basic Ontario tax = %v, want above the surtax T1 threshold for this test", sr.OntarioBasicTax)
		}
		if sr.OntarioBasicTaxAfterCredits >= sr.OntarioBasicTax {
			t.Fatalf("credits did not reduce basic Ontario tax: %v vs %v", sr.OntarioBasicTaxAfterCredits, sr.OntarioBasicTax)
		}
		almostEqual(t, sr.OntarioBasicTaxAfterCredits, 6193.827)
		almostEqual(t, sr.OntarioSurtax, 0.20*(6193.827-5818))
	}
}

func TestCompute_PensionSplitShiftsIncomeBetweenSpouses(t *testing.T) {
	unsplit, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", Age: 70, Income: Income{RRIFWithdrawals: 100000}},
		{Name: "B", Age: 70},
	}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	split, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", Age: 70, Income: Income{RRIFWithdrawals: 100000}, PensionSplitFraction: 0.5},
		{Name: "B", Age: 70},
	}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	almostEqual(t, split.Spouses[0].TaxableIncome, 50000)
	almostEqual(t, split.Spouses[1].TaxableIncome, 50000)
	almostEqual(t, split.Spouses[0].PensionTransferred, 50000)
	almostEqual(t, split.Spouses[1].PensionReceived, 50000)
	if split.Total >= unsplit.Total {
		t.Fatalf("split total %v should be below unsplit total %v", split.Total, unsplit.Total)
	}
}

func TestCompute_SpouseIncomeReducesSpousalCredit(t *testing.T) {
	low, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", Age: 50, Income: Income{RRIFWithdrawals: 150000}},
		{Name: "B", Age: 50},
	}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	high, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", Age: 50, Income: Income{RRIFWithdrawals: 150000}},
		{Name: "B", Age: 50, Income: Income{Interest: 20000}},
	}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if high.Spouses[0].FederalTax <= low.Spouses[0].FederalTax {
		t.Fatalf("A's federal tax with B earning 20000 = %v, want above %v (spousal amount reduced)",
			high.Spouses[0].FederalTax, low.Spouses[0].FederalTax)
	}
}

func TestCompute_EmploymentAmountCapsAtTableValue(t *testing.T) {
	// Employment income above the Canada employment amount (1501) earns no
	// further credit, so the tax difference between 5000 and 1501 of
	// employment income is the extra income at the lowest bracket. The RRIF
	// income puts A's tax above the credit floor so the delta is observable.
	above, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", Age: 50, Income: Income{RRIFWithdrawals: 50000, Employment: 5000}},
		{Name: "B", Age: 50},
	}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	atCap, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", Age: 50, Income: Income{RRIFWithdrawals: 50000, Employment: 1501}},
		{Name: "B", Age: 50},
	}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	delta := above.Spouses[0].FederalTax - atCap.Spouses[0].FederalTax
	almostEqual(t, delta, (5000-1501)*0.14)
}

func TestCompute_PensionAmountFollowsIncomeTypeAndAge(t *testing.T) {
	taxOf := func(age int, in Income) float64 {
		t.Helper()
		res, err := Compute(Household{Year: 2026, Spouses: []Spouse{
			{Name: "A", Age: age, Income: in},
			{Name: "B", Age: age, Income: Income{RRIFWithdrawals: 150000}},
		}})
		if err != nil {
			t.Fatalf("Compute: %v", err)
		}
		return res.Total
	}
	// RRIF income does not earn the pension amount before 65.
	if early, late := taxOf(64, Income{RRIFWithdrawals: 30000}), taxOf(65, Income{RRIFWithdrawals: 30000}); early <= late {
		t.Fatalf("RRIF at 64 taxed %v, want above the 65+ tax %v (pension amount starts at 65)", early, late)
	}
	// DB pension earns it at any age.
	if db, rrsp := taxOf(55, Income{DBPPension: 30000}), taxOf(55, Income{RRSPWithdrawals: 30000}); db >= rrsp {
		t.Fatalf("DB pension taxed %v, want below the RRSP tax %v (pension amount applies)", db, rrsp)
	}
	// RRSP withdrawals never earn it, even at 70.
	if rrif, rrsp := taxOf(70, Income{RRIFWithdrawals: 30000}), taxOf(70, Income{RRSPWithdrawals: 30000}); rrif >= rrsp {
		t.Fatalf("RRIF taxed %v, want below the RRSP tax %v", rrif, rrsp)
	}
	// CPP and OAS never earn it.
	if cpp, interest := taxOf(70, Income{CPP: 30000}), taxOf(70, Income{Interest: 30000}); cpp != interest {
		t.Fatalf("CPP taxed %v, want same as interest %v (no pension amount)", cpp, interest)
	}
	if oas, interest := taxOf(70, Income{OAS: 30000}), taxOf(70, Income{Interest: 30000}); oas != interest {
		t.Fatalf("OAS taxed %v, want same as interest %v (no pension amount)", oas, interest)
	}
}

func TestCompute_RejectsBadInput(t *testing.T) {
	if _, err := Compute(Household{Year: 2026, Spouses: []Spouse{{Name: "A"}}}); err == nil {
		t.Fatal("expected an error for a one-spouse household")
	}
	if _, err := Compute(Household{Year: 2026, Spouses: []Spouse{
		{Name: "A", PensionSplitFraction: 0.6},
		{Name: "B"},
	}}); err == nil {
		t.Fatal("expected an error for a split fraction above 50%")
	}
	if _, err := Compute(Household{Year: 2025, Spouses: []Spouse{{Name: "A"}, {Name: "B"}}}); err == nil {
		t.Fatal("expected an error for a year before the dated tables")
	}
}
