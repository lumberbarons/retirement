package projection

import (
	"math"
	"strings"
	"testing"

	"github.com/lumberbarons/retirement/internal/config"
	"github.com/lumberbarons/retirement/internal/constants"
	"github.com/lumberbarons/retirement/internal/tax"
)

const baseHousehold = `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 150000}
  - {name: Sam, birth_year: 1983, retirement_age: 62, employment_income: 90000}
accounts:
  - {name: Alex TFSA, type: tfsa, owner: Alex, balance: 100000}
  - {name: Alex RRSP, type: rrsp, owner: Alex, balance: 450000}
  - {name: Sam RRIF, type: rrif, owner: Sam, balance: 50000}
  - {name: Joint taxable, type: non_registered, owner: Alex, balance: 200000, acb: 150000}
spending: {target_today_dollars: 80000}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`

func mustHousehold(t *testing.T, yamlText string) *config.Household {
	t.Helper()
	h, err := config.Parse([]byte(yamlText))
	if err != nil {
		t.Fatalf("config parse: %v", err)
	}
	return h
}

func mustRun(t *testing.T, yamlText string, startYear int) []YearResult {
	t.Helper()
	results, err := Run(mustHousehold(t, yamlText), startYear)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Run returned no years")
	}
	return results
}

func accountYear(t *testing.T, res YearResult, name string) AccountYear {
	t.Helper()
	for _, a := range res.Accounts {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("year %d: no account %q", res.Year, name)
	return AccountYear{}
}

func TestRun_HorizonRunsToSecondDeath(t *testing.T) {
	results := mustRun(t, baseHousehold, 2026)
	if results[0].Year != 2026 {
		t.Fatalf("first year = %d, want 2026", results[0].Year)
	}
	if got := results[len(results)-1].Year; got != 2078 {
		t.Fatalf("last year = %d, want 2078 (second death: Sam 1983+95)", got)
	}
	if len(results) != 2078-2026+1 {
		t.Fatalf("year count = %d, want %d", len(results), 2078-2026+1)
	}
	if len(results[2076-2026].Deaths) != 1 || results[2076-2026].Deaths[0] != "Alex" {
		t.Fatalf("2076 deaths = %v, want [Alex]", results[2076-2026].Deaths)
	}
	if len(results[2078-2026].Deaths) != 1 || results[2078-2026].Deaths[0] != "Sam" {
		t.Fatalf("2078 deaths = %v, want [Sam]", results[2078-2026].Deaths)
	}
}

func TestRun_WorkingYearFundsTargetFromEmploymentIncome(t *testing.T) {
	results := mustRun(t, baseHousehold, 2026)
	first := results[0]
	if first.TargetNominal != 80000 {
		t.Fatalf("2026 target = %v, want 80000 (the target applies before retirement)", first.TargetNominal)
	}
	if first.EmploymentIncome != 240000 {
		t.Fatalf("2026 employment income = %v, want 240000", first.EmploymentIncome)
	}
	if first.Withdrawals != 0 {
		t.Fatalf("2026 discretionary withdrawals = %v, want 0 (earnings cover the target)", first.Withdrawals)
	}
	if first.NetSpending != first.TargetNominal {
		t.Fatalf("2026 net spending = %v, want the target %v", first.NetSpending, first.TargetNominal)
	}
	if first.Surplus <= 0 {
		t.Fatalf("2026 surplus = %v, want positive after-tax earnings above the target", first.Surplus)
	}
	wantMinimum := RoundCents(50000 * (1.0 / 48.0))
	if first.MandatoryIncome != wantMinimum {
		t.Fatalf("2026 mandatory = %v, want Sam's RRIF minimum %v (age at Jan 1 is 42)", first.MandatoryIncome, wantMinimum)
	}
	if first.BeginTotal != 800000 {
		t.Fatalf("2026 begin total = %v, want 800000", first.BeginTotal)
	}
	// The after-tax surplus lands in the non-registered account instead of
	// disappearing, so the household's ending wealth keeps it.
	taxable := accountYear(t, first, "Joint taxable")
	if want := RoundCents(200000*1.05) + first.Surplus; taxable.End != want {
		t.Fatalf("Joint taxable end = %v, want %v (grown balance plus retained surplus)", taxable.End, want)
	}
	if want := RoundCents(first.EmploymentIncome + first.MandatoryIncome - first.Tax - first.TargetNominal); first.Surplus != want {
		t.Fatalf("2026 surplus = %v, want after-tax earnings above target %v", first.Surplus, want)
	}
	wantEnd := RoundCents(100000*1.05) + RoundCents(450000*1.05) +
		RoundCents(50000*1.05-wantMinimum) + taxable.End
	if first.EndTotal != wantEnd {
		t.Fatalf("2026 end total = %v, want %v", first.EndTotal, wantEnd)
	}
}

func TestRun_Jan1SnapshotEqualsPriorYearEnd(t *testing.T) {
	results := mustRun(t, baseHousehold, 2026)
	for i := 1; i < len(results); i++ {
		if results[i].BeginTotal != results[i-1].EndTotal {
			t.Fatalf("year %d begin %v != prior year end %v", results[i].Year, results[i].BeginTotal, results[i-1].EndTotal)
		}
		for _, a := range results[i].Accounts {
			prior := accountYear(t, results[i-1], a.Name)
			if a.Begin != prior.End {
				t.Fatalf("year %d account %s begin %v != prior end %v", results[i].Year, a.Name, a.Begin, prior.End)
			}
		}
	}
}

func TestRun_RetirementYearProratesEmploymentIncome(t *testing.T) {
	results := mustRun(t, baseHousehold, 2026)
	wage := 1.031
	year2041 := results[2041-2026]
	want2041 := RoundCents(0.5*150000*math.Pow(wage, 15)) + RoundCents(90000*math.Pow(wage, 15))
	if math.Abs(year2041.EmploymentIncome-want2041) > 0.005 {
		t.Fatalf("2041 employment income = %v, want %v (Alex's half year plus Sam's full year)", year2041.EmploymentIncome, want2041)
	}
	if want := RoundCents(80000 * math.Pow(1.021, 15)); year2041.TargetNominal != want {
		t.Fatalf("2041 target = %v, want the full-year %v (spending does not start at retirement)", year2041.TargetNominal, want)
	}
	if want := RoundCents(90000 * math.Pow(wage, 16)); math.Abs(results[2042-2026].EmploymentIncome-want) > 0.005 {
		t.Fatalf("2042 employment income = %v, want only Sam's %v", results[2042-2026].EmploymentIncome, want)
	}
	if want := RoundCents(0.5 * 90000 * math.Pow(wage, 19)); math.Abs(results[2045-2026].EmploymentIncome-want) > 0.005 {
		t.Fatalf("2045 employment income = %v, want Sam's half year %v", results[2045-2026].EmploymentIncome, want)
	}
	if results[2046-2026].EmploymentIncome != 0 {
		t.Fatalf("2046 employment income = %v, want 0 once both spouses are retired", results[2046-2026].EmploymentIncome)
	}
}

func TestRun_EmploymentIncomeGrowsWithWageGrowth(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 100000}
  - {name: Sam, birth_year: 1983, retirement_age: 62}
accounts:
  - {name: Savings, type: non_registered, owner: Alex, balance: 1000000}
spending: {target_today_dollars: 50000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021, wage_growth: 0.04}
`
	results := mustRun(t, yamlText, 2026)
	if results[0].EmploymentIncome != 100000 {
		t.Fatalf("2026 employment income = %v, want 100000", results[0].EmploymentIncome)
	}
	if want := RoundCents(100000 * math.Pow(1.04, 10)); results[10].EmploymentIncome != want {
		t.Fatalf("2036 employment income = %v, want %v (4%% wage growth)", results[10].EmploymentIncome, want)
	}
}

func TestRun_EarningsFundTargetBeforeWithdrawals(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 30000}
  - {name: Sam, birth_year: 1983, retirement_age: 62}
accounts:
  - {name: Savings, type: non_registered, owner: Alex, balance: 500000}
spending: {target_today_dollars: 60000, mode: flat}
assumptions: {portfolio_return: 0.055, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	first := results[0]
	if first.EmploymentIncome != 30000 {
		t.Fatalf("2026 employment income = %v, want 30000", first.EmploymentIncome)
	}
	if first.NetSpending != first.TargetNominal {
		t.Fatalf("2026 net spending = %v, want the target %v", first.NetSpending, first.TargetNominal)
	}
	if first.Withdrawals <= 0 {
		t.Fatalf("2026 withdrawals = %v, want the earnings shortfall drawn from the portfolio", first.Withdrawals)
	}
	if first.Surplus != 0 {
		t.Fatalf("2026 surplus = %v, want 0 when earnings fall short of the target", first.Surplus)
	}
}

func TestStepEmploymentIncome_AttributesToTheEarner(t *testing.T) {
	h := mustHousehold(t, `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 100000}
  - {name: Sam, birth_year: 1983, retirement_age: 62}
accounts:
  - {name: Savings, type: non_registered, owner: Alex, balance: 1000}
spending: {target_today_dollars: 50000}
`)
	s := NewState(h, 2026)
	incomes := make([]tax.Income, len(s.People))
	var res YearResult
	s.stepEmploymentIncome(h, incomes, &res)
	if incomes[0].Employment != 100000 {
		t.Fatalf("Alex employment = %v, want 100000", incomes[0].Employment)
	}
	if incomes[1].Employment != 0 {
		t.Fatalf("Sam employment = %v, want 0", incomes[1].Employment)
	}
	if res.EmploymentIncome != 100000 {
		t.Fatalf("year employment income = %v, want 100000", res.EmploymentIncome)
	}
}

func TestRun_EmploymentIncomeEntersTaxableIncome(t *testing.T) {
	const noEarnings = `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 65}
  - {name: Sam, birth_year: 1983, retirement_age: 65}
accounts:
  - {name: Savings, type: non_registered, owner: Alex, balance: 2000000}
spending: {target_today_dollars: 40000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	withEarnings := mustRun(t, strings.Replace(noEarnings,
		"retirement_age: 65}", "retirement_age: 65, employment_income: 80000}", 1), 2026)
	without := mustRun(t, noEarnings, 2026)
	if withEarnings[0].TaxableIncome <= without[0].TaxableIncome {
		t.Fatalf("taxable income with earnings = %v, want above %v without",
			withEarnings[0].TaxableIncome, without[0].TaxableIncome)
	}
	if withEarnings[0].Tax <= without[0].Tax {
		t.Fatalf("tax with earnings = %v, want above %v without", withEarnings[0].Tax, without[0].Tax)
	}
}

func TestStepYear_RetainedSurplusRaisesNonRegisteredACB(t *testing.T) {
	h := mustHousehold(t, `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 100000}
  - {name: Sam, birth_year: 1983, retirement_age: 62}
accounts:
  - {name: Savings, type: non_registered, owner: Alex, balance: 100000, acb: 80000}
spending: {target_today_dollars: 40000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`)
	s := NewState(h, 2026)
	res, err := s.stepYear(h)
	if err != nil {
		t.Fatalf("stepYear: %v", err)
	}
	if res.Surplus <= 0 {
		t.Fatalf("surplus = %v, want positive", res.Surplus)
	}
	if want := 80000 + res.Surplus; s.Accounts[0].ACB != want {
		t.Fatalf("ACB = %v, want opening ACB plus retained surplus %v", s.Accounts[0].ACB, want)
	}
	if want := RoundCents(100000*1.05) + res.Surplus; res.EndTotal != want {
		t.Fatalf("end total = %v, want %v (the surplus stays in the household)", res.EndTotal, want)
	}
}

func TestRun_RRIFMinimumUsesJan1SnapshotAndAge(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Pat, birth_year: 1982, retirement_age: 60}
  - {name: Mel, birth_year: 1984, retirement_age: 62}
accounts:
  - {name: Pat RRIF, type: rrif, owner: Pat, balance: 500000}
  - {name: Mel non-registered, type: non_registered, owner: Mel, balance: 1000000}
spending: {target_today_dollars: 60000}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	year2 := results[1]
	rrif := accountYear(t, year2, "Pat RRIF")
	want2026Minimum := RoundCents(500000 * (1.0 / 47.0))
	if rrif.Begin != RoundCents(500000*1.05-want2026Minimum) {
		t.Fatalf("2027 RRIF begin = %v, want %v (2026 minimum on Jan-1 balance, withdrawn post-return)", rrif.Begin, RoundCents(500000*1.05-want2026Minimum))
	}
	want := RoundCents(rrif.Begin * (1.0 / 46.0))
	if year2.MandatoryIncome != want {
		t.Fatalf("2027 RRIF minimum = %v, want %v (age at Jan 1 is 44: 1/(90-44))", year2.MandatoryIncome, want)
	}
	year2054 := results[2054-2026]
	rrif2054 := accountYear(t, year2054, "Pat RRIF")
	if year2054.MandatoryIncome != RoundCents(rrif2054.Begin*0.0528) {
		t.Fatalf("2054 RRIF minimum = %v, want %v (age at Jan 1 is 71: 5.28%% of Jan-1 balance)", year2054.MandatoryIncome, RoundCents(rrif2054.Begin*0.0528))
	}
}

func TestRun_RRIFMinimumUsesYoungerSpouseAgeWhenElected(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, death_age: 99}
  - {name: Sam, birth_year: 1983, retirement_age: 62, death_age: 90}
accounts:
  - {name: Alex RRIF, type: rrif, owner: Alex, balance: 50000, younger_spouse_election: true}
  - {name: Sam non-registered, type: non_registered, owner: Sam, balance: 5000000}
spending: {target_today_dollars: 60000}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	first := results[0]
	// Alex is 44 at Jan 1 2026 (factor 1/(90-44)); the election drops the
	// factor to Sam's age 42: 1/(90-42).
	want := RoundCents(50000 * (1.0 / 48.0))
	if first.MandatoryIncome != want {
		t.Fatalf("2026 mandatory = %v, want %v (elected to Sam's age 42, not Alex's 44)", first.MandatoryIncome, want)
	}
	// The election keeps using Sam's would-be age even after Sam dies:
	// Sam dies in 2073, so 2074 minimums still use Sam's age at Jan 1 (90).
	year2074 := results[2074-2026]
	rrif := accountYear(t, year2074, "Alex RRIF")
	if rrif.Begin <= 0 {
		t.Fatalf("2074 RRIF begin = %v, want a positive balance to observe the minimum", rrif.Begin)
	}
	want2074 := RoundCents(rrif.Begin * constants.RRIFMinimumFactor(2074-1983-1))
	if year2074.MandatoryIncome != want2074 {
		t.Fatalf("2074 mandatory = %v, want %v (Sam's would-be age 90)", year2074.MandatoryIncome, want2074)
	}
}

func TestRun_WithdrawalPriorityOrder(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 40}
  - {name: Sam, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: RRSP, type: rrsp, owner: Alex, balance: 500000}
  - {name: TFSA, type: tfsa, owner: Sam, balance: 100000}
  - {name: Taxable, type: non_registered, owner: Alex, balance: 10000}
spending: {target_today_dollars: 30000}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	first := results[0]
	if first.TargetNominal != 30000 {
		t.Fatalf("target = %v, want 30000 (already retired)", first.TargetNominal)
	}
	if got := accountYear(t, first, "Taxable"); got.End != 0 || got.Withdrawal != 10500 {
		t.Fatalf("taxable = %+v, want drained: withdrawal 10500 end 0", got)
	}
	rrsp := accountYear(t, first, "RRSP")
	if rrsp.Withdrawal != 19500 || rrsp.End != RoundCents(500000*1.05-19500) {
		t.Fatalf("rrsp = %+v, want withdrawal 19500 after taxable drains", rrsp)
	}
	tfsa := accountYear(t, first, "TFSA")
	if tfsa.Withdrawal != 0 || tfsa.End != RoundCents(100000*1.05) {
		t.Fatalf("tfsa = %+v, want untouched at %v", tfsa, RoundCents(100000*1.05))
	}
}

func TestRun_ExhaustedAccountsFallShort(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 40}
  - {name: Sam, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: Taxable, type: non_registered, owner: Alex, balance: 1000}
spending: {target_today_dollars: 50000}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	first := results[0]
	if first.NetSpending != RoundCents(1000*1.05) {
		t.Fatalf("net spending = %v, want %v (everything withdrawn)", first.NetSpending, RoundCents(1000*1.05))
	}
	if first.NetSpending >= first.TargetNominal {
		t.Fatalf("net spending %v should fall short of target %v", first.NetSpending, first.TargetNominal)
	}
	if first.EndTotal != 0 {
		t.Fatalf("end total = %v, want 0", first.EndTotal)
	}
	second := results[1]
	if second.NetSpending != 0 || second.EndTotal != 0 {
		t.Fatalf("2054... second year = %v/%v, want 0/0", second.NetSpending, second.EndTotal)
	}
}

func TestRun_SpendingConvertsRealToNominal(t *testing.T) {
	results := mustRun(t, baseHousehold, 2026)
	year2046 := results[2046-2026]
	want := 80000 * math.Pow(1.021, 20)
	if math.Abs(year2046.TargetNominal-want) > 1 {
		t.Fatalf("2046 target = %v, want ~121000 (80000 at 2.1%% for 20 years), tolerance $1", year2046.TargetNominal)
	}
	if year2046.NetSpending != year2046.TargetNominal {
		t.Fatalf("2046 net spending = %v, want funded target %v", year2046.NetSpending, year2046.TargetNominal)
	}
}

func TestRun_RejectsInvalidConfig(t *testing.T) {
	h := mustHousehold(t, baseHousehold)
	h.Spouses = h.Spouses[:1]
	if _, err := Run(h, 2026); err == nil {
		t.Fatal("Run should reject an invalid household")
	}
}
