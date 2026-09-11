// Package validate ports every validation case in the governing spec
// (ontario-retirement-simulator-spec.md §7) into a named test. Each case must
// land within ±$1, the gate the deterministic engine must pass before any
// later epic starts.
//
// The §7 case "Spousal RRIF minimum withdrawal within the 3-year window
// attributes $0 to the contributor" needs the spousal attribution engine,
// which does not exist yet — it lands with the account-mechanics issue and is
// tracked separately as #26. The CPP and OAS cases are asserted here against
// the dated tables; their behavioural halves land with the benefits engine,
// tracked as #28.
package validate

import (
	"math"
	"testing"

	"github.com/lumberbarons/retirement/internal/config"
	"github.com/lumberbarons/retirement/internal/constants"
	"github.com/lumberbarons/retirement/internal/projection"
	"github.com/lumberbarons/retirement/internal/tax"
)

func almostEqual(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1 {
		t.Fatalf("got %v, want %v within $1", got, want)
	}
}

func parse(t *testing.T, yamlText string) *config.Household {
	t.Helper()
	h, err := config.Parse([]byte(yamlText))
	if err != nil {
		t.Fatalf("config.Parse: %v", err)
	}
	return h
}

func runProjection(t *testing.T, yamlText string) []projection.YearResult {
	t.Helper()
	results, err := projection.Run(parse(t, yamlText), 2026)
	if err != nil {
		t.Fatalf("projection.Run: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("projection.Run returned no years")
	}
	return results
}

// TestValidation_RRIFMinimums covers §7: "RRIF minimum at 71 on $500k Jan-1 =
// $26,400 (5.28%); at 72 = 5.40%."
func TestValidation_RRIFMinimums(t *testing.T) {
	almostEqual(t, constants.RRIFMinimumFactor(71)*500000, 26400)
	if factor := constants.RRIFMinimumFactor(72); math.Abs(factor-0.0540) > 1e-9 {
		t.Fatalf("RRIF factor at 72 = %v, want 0.0540", factor)
	}
}

// TestValidation_OASFullClawbackCeilings covers §7: the OAS recovery tax
// reclaims the full pension at the ESDC ceilings ($154,708 at 65–74, $160,647
// at 75+). The table figures must reconcile with the threshold and the rate.
func TestValidation_OASFullClawbackCeilings(t *testing.T) {
	row, _, err := constants.OAS.For(2026)
	if err != nil {
		t.Fatalf("constants.OAS.For: %v", err)
	}
	rate, err := constants.OASRecoveryRate.For(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("constants.OASRecoveryRate.For: %v", err)
	}
	cases := []struct {
		name    string
		monthly float64
		ceiling float64
	}{
		{"65-74", row.Monthly65to74, row.ClawbackCeiling65to74},
		{"75+", row.Monthly75Plus, row.ClawbackCeiling75Plus},
	}
	for _, c := range cases {
		recoveryAtCeiling := rate * (c.ceiling - row.ClawbackThreshold)
		almostEqual(t, recoveryAtCeiling, 12*c.monthly)
	}
}

// TestValidation_FullOASAmounts covers §7: full OAS is $742.31/mo (65–74) and
// $816.54/mo (75+) at 40 years' residence. The residence scaling itself is
// modelled by the government-benefits issue; the published rates are asserted
// here.
func TestValidation_FullOASAmounts(t *testing.T) {
	row, _, err := constants.OAS.For(2026)
	if err != nil {
		t.Fatalf("constants.OAS.For: %v", err)
	}
	if row.Monthly65to74 != 742.31 || row.Monthly75Plus != 816.54 {
		t.Fatalf("OAS rates = %v/%v, want 742.31/816.54", row.Monthly65to74, row.Monthly75Plus)
	}
}

// TestValidation_CPPStartAgeAdjustment covers §7: CPP at 60 is 0.640x the
// age-65 entitlement, at 70 it is 1.420x, built from the monthly actuarial
// adjustment rates.
func TestValidation_CPPStartAgeAdjustment(t *testing.T) {
	early, err := constants.CPPEarlyAdjustmentMonthly.For(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("constants.CPPEarlyAdjustmentMonthly.For: %v", err)
	}
	late, err := constants.CPPLateAdjustmentMonthly.For(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("constants.CPPLateAdjustmentMonthly.For: %v", err)
	}
	if at60 := 1 - early*5*12; math.Abs(at60-0.640) > 0.001 {
		t.Fatalf("CPP factor at 60 = %v, want 0.640", at60)
	}
	if at70 := 1 + late*5*12; math.Abs(at70-1.420) > 0.001 {
		t.Fatalf("CPP factor at 70 = %v, want 1.420", at70)
	}
}

// TestValidation_FederalBPACredit covers §7: the $16,452 federal BPA is worth
// roughly $2,303 at the lowest rate.
func TestValidation_FederalBPACredit(t *testing.T) {
	bpa, err := tax.FederalBasicPersonalAmount(0, 2026, constants.Forward{})
	if err != nil {
		t.Fatalf("tax.FederalBasicPersonalAmount: %v", err)
	}
	rate, err := tax.FederalCreditRate(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("tax.FederalCreditRate: %v", err)
	}
	almostEqual(t, bpa*rate, 2303)
}

// TestValidation_CPPSurvivorCombinedCap covers §7: the combined survivor and
// own retirement CPP is capped at $1,531.56/mo.
func TestValidation_CPPSurvivorCombinedCap(t *testing.T) {
	row, _, err := constants.CPP.For(2026)
	if err != nil {
		t.Fatalf("constants.CPP.For: %v", err)
	}
	if row.SurvivorCap != 1531.56 {
		t.Fatalf("survivor combined cap = %v, want 1531.56", row.SurvivorCap)
	}
}

// TestValidation_OntarioSurtaxThresholds covers §7: the 20% surtax begins once
// basic Ontario tax after credits exceeds $5,818, and the 36% tier at $7,446.
func TestValidation_OntarioSurtaxThresholds(t *testing.T) {
	if got := tax.Surtax(5818, 5818, 7446); got != 0 {
		t.Fatalf("surtax at T1 = %v, want 0", got)
	}
	almostEqual(t, tax.Surtax(7446, 5818, 7446), 0.20*(7446-5818))
}

// TestValidation_OntarioBasicTax covers §7: basic Ontario tax on $107,785 of
// taxable income, before credits, is $7,652.80
// (53,891 x 5.05% + 53,894 x 9.15%).
func TestValidation_OntarioBasicTax(t *testing.T) {
	brackets, err := tax.OntarioBrackets(2026, constants.Forward{})
	if err != nil {
		t.Fatalf("tax.OntarioBrackets: %v", err)
	}
	almostEqual(t, tax.BracketTax(107785, brackets), 53891*0.0505+53894*0.0915)
}

// TestValidation_OntarioHealthPremium covers §7's premium plateaus and ramps.
func TestValidation_OntarioHealthPremium(t *testing.T) {
	row, _, err := constants.Tax.For(2026)
	if err != nil {
		t.Fatalf("constants.Tax.For: %v", err)
	}
	cases := []struct {
		taxable float64
		want    float64
	}{
		{30000, 300},
		{37000, 360},
		{48300, 525},
		{72300, 675},
		{250000, 900},
	}
	for _, c := range cases {
		almostEqual(t, tax.HealthPremium(c.taxable, row.ONHealthPremium), c.want)
	}
}

// TestValidation_SpendingRealToNominal covers §7: $80,000 real at 2.1%
// inflation is about $121,000 nominal in 20 years.
func TestValidation_SpendingRealToNominal(t *testing.T) {
	results := runProjection(t, `base_year: 2026
spouses:
  - {name: A, birth_year: 1981, retirement_age: 40}
  - {name: B, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: A RRSP, type: rrsp, owner: A, balance: 2000000}
spending: {target_today_dollars: 80000, mode: flat, inflation: 0.021}
assumptions: {portfolio_return: 0.05, inflation: 0.021, wage_growth: 0.031}
`)
	year := results[2046-2026]
	want := 80000 * math.Pow(1.021, 20)
	almostEqual(t, year.TargetNominal, want)
	if math.Abs(year.TargetNominal-121000) > 500 {
		t.Fatalf("2046 nominal target = %v, want about 121000", year.TargetNominal)
	}
}

// TestValidation_GrossVsNetSolve covers §7: a net target of $80,000 drawn
// wholly from a RRIF converges to a gross figure whose computed net is within
// $1, both through the bisection directly and through a full projection year.
func TestValidation_GrossVsNetSolve(t *testing.T) {
	netAfterTax := func(gross float64) float64 {
		res, err := tax.Compute(tax.Household{Year: 2026, Spouses: []tax.Spouse{
			{Name: "A", Age: 70, Income: tax.Income{RRIFWithdrawals: gross}},
			{Name: "B", Age: 70},
		}})
		if err != nil {
			t.Fatalf("tax.Compute: %v", err)
		}
		return gross - res.Total
	}
	gross := projection.SolveGross(80000, 1000000, netAfterTax)
	almostEqual(t, netAfterTax(gross), 80000)
	if gross <= 80000 {
		t.Fatalf("gross withdrawal = %v, want above the 80000 net target once tax is paid", gross)
	}

	results := runProjection(t, `base_year: 2026
spouses:
  - {name: A, birth_year: 1956, retirement_age: 40}
  - {name: B, birth_year: 1956, retirement_age: 40}
accounts:
  - {name: A RRIF, type: rrif, owner: A, balance: 1000000}
spending: {target_today_dollars: 80000, mode: flat}
assumptions: {portfolio_return: 0.05}
`)
	first := results[0]
	if first.Tax <= 0 {
		t.Fatalf("tax = %v, want positive on RRIF withdrawals", first.Tax)
	}
	almostEqual(t, first.NetSpending, first.TargetNominal)
}

// TestValidation_DividendGrossUpsAndCapitalGainsInclusion covers the done-when
// requirement that eligible and non-eligible dividends apply their own gross-up
// and dividend tax credit, and that capital gains include at 50%.
func TestValidation_DividendGrossUpsAndCapitalGainsInclusion(t *testing.T) {
	compute := func(in tax.Income) tax.Result {
		t.Helper()
		res, err := tax.Compute(tax.Household{Year: 2026, Spouses: []tax.Spouse{
			{Name: "A", Age: 50, Income: in},
			{Name: "B", Age: 50},
		}})
		if err != nil {
			t.Fatalf("tax.Compute: %v", err)
		}
		return res
	}
	eligible := compute(tax.Income{EligibleDividends: 10000})
	almostEqual(t, eligible.Spouses[0].TaxableIncome, 13800)
	almostEqual(t, eligible.Spouses[0].FederalDividendTaxCredit, 13800*0.150198)
	almostEqual(t, eligible.Spouses[0].OntarioDividendTaxCredit, 1380)

	nonEligible := compute(tax.Income{NonEligibleDividends: 10000})
	almostEqual(t, nonEligible.Spouses[0].TaxableIncome, 11500)
	almostEqual(t, nonEligible.Spouses[0].FederalDividendTaxCredit, 11500*0.090301)
	almostEqual(t, nonEligible.Spouses[0].OntarioDividendTaxCredit, 11500*0.029863)

	gains := compute(tax.Income{CapitalGains: 10000})
	almostEqual(t, gains.Spouses[0].TaxableIncome, 5000)
}

// TestValidation_CapitalGainsRealizedAgainstACB covers the done-when
// requirement that a non-registered withdrawal realizes capital gains
// proportional to (1 - ACB/balance) at 50% inclusion.
func TestValidation_CapitalGainsRealizedAgainstACB(t *testing.T) {
	results := runProjection(t, `base_year: 2026
spouses:
  - {name: A, birth_year: 1981, retirement_age: 40}
  - {name: B, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: A non-registered, type: non_registered, owner: A, balance: 200000, acb: 150000}
spending: {target_today_dollars: 40000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`)
	first := results[0]
	// The 5% return moves the balance to 210,000 while ACB stays 150,000, so
	// the gain fraction is (1 - 150000/210000) and half of it is included.
	want := 0.5 * first.Withdrawals * (1 - 150000.0/210000)
	if math.Abs(first.TaxableIncome-want) > 1 {
		t.Fatalf("taxable income = %v, want capital-gains inclusion %v", first.TaxableIncome, want)
	}
}

// TestValidation_UnusedCreditsTransferToHigherIncomeSpouse covers Schedule 2:
// the lower-income spouse's unused age and pension credits transfer to the
// higher-income spouse.
func TestValidation_UnusedCreditsTransferToHigherIncomeSpouse(t *testing.T) {
	res, err := tax.Compute(tax.Household{Year: 2026, Spouses: []tax.Spouse{
		{Name: "A", Age: 70, Income: tax.Income{RRIFWithdrawals: 120000}},
		{Name: "B", Age: 70, Income: tax.Income{RRIFWithdrawals: 5000}},
	}})
	if err != nil {
		t.Fatalf("tax.Compute: %v", err)
	}
	b := res.Spouses[1]
	if b.CreditTransferredOut <= 0 {
		t.Fatal("the lower-income spouse transferred no credits")
	}
	if res.Spouses[0].CreditTransferredIn != b.CreditTransferredOut {
		t.Fatalf("A transferred in %v, want B's transferred out %v",
			res.Spouses[0].CreditTransferredIn, b.CreditTransferredOut)
	}
	// B's unused age and pension amounts at the published 2026 figures.
	want := (9208+2000)*0.14 + (6342+1796)*0.0505
	almostEqual(t, b.CreditTransferredOut, want)
}

// TestValidation_T1032SplitShiftsIncome covers the done-when requirement that
// a user-set T1032 split fraction shifts pension income between spouses, and
// that the default fraction of 0 changes nothing.
func TestValidation_T1032SplitShiftsIncome(t *testing.T) {
	const yamlText = `base_year: 2026
spouses:
  - {name: A, birth_year: 1956, retirement_age: 40, pension_split: [{year: 2026, fraction: 0.5}]}
  - {name: B, birth_year: 1956, retirement_age: 40}
accounts:
  - {name: A RRIF, type: rrif, owner: A, balance: 1000000}
spending: {target_today_dollars: 80000, mode: flat}
assumptions: {portfolio_return: 0.05}
`
	withSplit := runProjection(t, yamlText)

	without := parse(t, yamlText)
	without.Spouses[0].PensionSplit = nil
	results, err := projection.Run(without, 2026)
	if err != nil {
		t.Fatalf("projection.Run: %v", err)
	}
	if withSplit[0].Tax >= results[0].Tax {
		t.Fatalf("split tax %v, want below the unsplit tax %v (income moved to the lower earner)",
			withSplit[0].Tax, results[0].Tax)
	}

	// The default fraction of 0 must reproduce the unsplit result exactly.
	defaulted, err := tax.Compute(tax.Household{Year: 2026, Spouses: []tax.Spouse{
		{Name: "A", Age: 70, Income: tax.Income{RRIFWithdrawals: 100000}},
		{Name: "B", Age: 70},
	}})
	if err != nil {
		t.Fatalf("tax.Compute: %v", err)
	}
	explicitZero, err := tax.Compute(tax.Household{Year: 2026, Spouses: []tax.Spouse{
		{Name: "A", Age: 70, Income: tax.Income{RRIFWithdrawals: 100000}, PensionSplitFraction: 0},
		{Name: "B", Age: 70, PensionSplitFraction: 0},
	}})
	if err != nil {
		t.Fatalf("tax.Compute: %v", err)
	}
	if defaulted.Total != explicitZero.Total {
		t.Fatalf("default split total %v differs from an explicit zero %v", defaulted.Total, explicitZero.Total)
	}
	if defaulted.Spouses[0].PensionTransferred != 0 || defaulted.Spouses[1].PensionReceived != 0 {
		t.Fatalf("default split moved pension: %+v", defaulted.Spouses)
	}
}
