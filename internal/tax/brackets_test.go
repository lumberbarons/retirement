package tax

import (
	"math"
	"testing"

	"github.com/lumberbarons/retirement/internal/constants"
)

func almostEqual(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func constantsIncome(t *testing.T) (constants.IncomeYear, int, error) {
	t.Helper()
	return constants.Income.For(2026)
}

func TestBracketTax_ProgressiveAcrossBrackets(t *testing.T) {
	brackets, err := FederalBrackets(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("FederalBrackets: %v", err)
	}
	almostEqual(t, BracketTax(58523, brackets), 58523*0.14)
	almostEqual(t, BracketTax(117045, brackets), 58523*0.14+(117045-58523)*0.205)
	if got := BracketTax(0, brackets); got != 0 {
		t.Fatalf("tax on zero income = %v, want 0", got)
	}
	if got := BracketTax(-500, brackets); got != 0 {
		t.Fatalf("tax on negative income = %v, want 0", got)
	}
}

func TestBracketTax_AboveTopBracket(t *testing.T) {
	brackets, err := FederalBrackets(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("FederalBrackets: %v", err)
	}
	want := 58523*0.14 +
		(117045-58523)*0.205 +
		(181440-117045)*0.26 +
		(258482-181440)*0.29 +
		(300000-258482)*0.33
	almostEqual(t, BracketTax(300000, brackets), want)
}

func TestIndexBrackets_MixedBases(t *testing.T) {
	brackets, err := OntarioBrackets(2027, constants.Forward{CPI: 0.02})
	if err != nil {
		t.Fatalf("OntarioBrackets: %v", err)
	}
	almostEqual(t, brackets[0].Upper, 53891*1.02)
	almostEqual(t, brackets[1].Upper, 107785*1.02)
	if brackets[2].Lower != 107785 || brackets[2].Upper != 150000 {
		t.Fatalf("frozen bracket = %v-%v, want 107785-150000 unchanged", brackets[2].Lower, brackets[2].Upper)
	}
	if brackets[4].Upper != math.Inf(1) {
		t.Fatalf("top bracket upper = %v, want +Inf", brackets[4].Upper)
	}
}

func TestFederalBrackets_IndexedForFutureYear(t *testing.T) {
	brackets, err := FederalBrackets(2027, constants.Forward{CPI: 0.02})
	if err != nil {
		t.Fatalf("FederalBrackets: %v", err)
	}
	almostEqual(t, brackets[0].Upper, 58523*1.02)
	almostEqual(t, brackets[1].Lower, 58523*1.02)
}

func TestIndexYear_YearBeforeTablesErrors(t *testing.T) {
	if _, err := FederalBrackets(2025, constants.Forward{}); err == nil {
		t.Fatal("expected an error for a year before the dated tables")
	}
}

func TestFederalBasicPersonalAmount_PhasesOut(t *testing.T) {
	bpa := func(taxable float64) float64 {
		got, err := FederalBasicPersonalAmount(taxable, 2026, constants.Forward{})
		if err != nil {
			t.Fatalf("FederalBasicPersonalAmount: %v", err)
		}
		return got
	}
	almostEqual(t, bpa(100000), 16452)
	almostEqual(t, bpa(181440), 16452)
	almostEqual(t, bpa(258482), 14829)
	almostEqual(t, bpa(300000), 14829)
	almostEqual(t, bpa((181440+258482)/2.0), (16452+14829)/2.0)
}

func TestCreditRates_AreLowestBracketRates(t *testing.T) {
	fed, err := FederalCreditRate(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("FederalCreditRate: %v", err)
	}
	on, err := OntarioCreditRate(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("OntarioCreditRate: %v", err)
	}
	almostEqual(t, fed, 0.14)
	almostEqual(t, on, 0.0505)
}

func TestAgeAmount_PhasesOutAndStopsAtNil(t *testing.T) {
	almostEqual(t, ageAmount(40000, 70, 9208, 46432, 107819), 9208)
	almostEqual(t, ageAmount(46432, 70, 9208, 46432, 107819), 9208)
	almostEqual(t, ageAmount(107819, 70, 9208, 46432, 107819), 0)
	almostEqual(t, ageAmount(100000, 70, 9208, 46432, 107819), 9208-0.15*(100000-46432))
	if got := ageAmount(100000, 64, 9208, 46432, 107819); got != 0 {
		t.Fatalf("age amount under 65 = %v, want 0", got)
	}
}

func TestSpousalAmount_ReducesWithSpouseIncome(t *testing.T) {
	almostEqual(t, spousalAmount(0, 16452, 0), 16452)
	almostEqual(t, spousalAmount(5000, 16452, 0), 11452)
	almostEqual(t, spousalAmount(20000, 16452, 0), 0)
	almostEqual(t, spousalAmount(1000, 11029, 1103), 11029)
	almostEqual(t, spousalAmount(5000, 11029, 1103), 11029-3897)
}
