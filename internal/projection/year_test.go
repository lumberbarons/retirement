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
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 150000, savings_account: Joint taxable}
  - {name: Sam, birth_year: 1983, retirement_age: 62, employment_income: 90000, savings_account: Joint taxable}
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
	if want := RoundCents(2 * (71100*0.0595 + 10400*0.04)); first.CPPContributions != want {
		t.Fatalf("2026 CPP contributions = %v, want %v (both spouses at the YAMPE cap)", first.CPPContributions, want)
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
	if want := RoundCents(first.EmploymentIncome + first.MandatoryIncome - first.Tax -
		first.CPPContributions - first.TargetNominal); first.Surplus != want {
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
	if err := s.stepEmploymentIncome(h, incomes, &res); err != nil {
		t.Fatalf("stepEmploymentIncome: %v", err)
	}
	if incomes[0].Employment != 100000 {
		t.Fatalf("Alex employment = %v, want 100000", incomes[0].Employment)
	}
	if incomes[1].Employment != 0 {
		t.Fatalf("Sam employment = %v, want 0", incomes[1].Employment)
	}
	if res.EmploymentIncome != 100000 {
		t.Fatalf("year employment income = %v, want 100000", res.EmploymentIncome)
	}
	if want := RoundCents(71100*0.0595 + 10400*0.04); incomes[0].CPPContributions != want {
		t.Fatalf("Alex CPP contributions = %v, want %v", incomes[0].CPPContributions, want)
	}
	if incomes[1].CPPContributions != 0 {
		t.Fatalf("Sam CPP contributions = %v, want 0 with no earnings", incomes[1].CPPContributions)
	}
	if res.CPPContributions != incomes[0].CPPContributions {
		t.Fatalf("year CPP contributions = %v, want the earner's %v", res.CPPContributions, incomes[0].CPPContributions)
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

// TestRun_SurplusStaysWithItsEarnerWhenAccountsAreReversed covers surplus
// ownership: with one taxable account per spouse, each spouse's share lands
// in their own account even though the other spouse's account is listed first
// in config order.
func TestRun_SurplusStaysWithItsEarnerWhenAccountsAreReversed(t *testing.T) {
	results := mustRun(t, `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 150000}
  - {name: Sam, birth_year: 1983, retirement_age: 62, employment_income: 60000}
accounts:
  - {name: Sam taxable, type: non_registered, owner: Sam, balance: 100000, acb: 90000}
  - {name: Alex taxable, type: non_registered, owner: Alex, balance: 200000, acb: 150000}
spending: {target_today_dollars: 70000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`, 2026)
	first := results[0]
	alex, sam := first.Spouses[0], first.Spouses[1]
	if alex.Surplus <= 0 || sam.Surplus <= 0 {
		t.Fatalf("spouse surplus = %v/%v, want both positive", alex.Surplus, sam.Surplus)
	}
	if alex.Surplus <= sam.Surplus {
		t.Fatalf("Alex surplus %v should exceed Sam's %v on the higher income", alex.Surplus, sam.Surplus)
	}
	if first.UnallocatedSurplus != 0 {
		t.Fatalf("unallocated surplus = %v, want 0 with an owned account each", first.UnallocatedSurplus)
	}
	alexAccount := accountYear(t, first, "Alex taxable")
	if want := RoundCents(200000*1.05) + alex.Surplus; alexAccount.End != want {
		t.Fatalf("Alex taxable end = %v, want %v (only Alex's share)", alexAccount.End, want)
	}
	samAccount := accountYear(t, first, "Sam taxable")
	if want := RoundCents(100000*1.05) + sam.Surplus; samAccount.End != want {
		t.Fatalf("Sam taxable end = %v, want %v (only Sam's share)", samAccount.End, want)
	}
	if want := alexAccount.End + samAccount.End; first.EndTotal != want {
		t.Fatalf("end total = %v, want the two taxable accounts summed %v", first.EndTotal, want)
	}
}

// TestRun_SavingsAccountRoutesSurplusExplicitly covers the declared
// destination: a spouse can route their surplus into a jointly held account
// modelled under the other spouse's name.
func TestRun_SavingsAccountRoutesSurplusExplicitly(t *testing.T) {
	results := mustRun(t, `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 150000, savings_account: Joint taxable}
  - {name: Sam, birth_year: 1983, retirement_age: 62, employment_income: 90000, savings_account: Joint taxable}
accounts:
  - {name: Joint taxable, type: non_registered, owner: Alex, balance: 200000, acb: 150000}
spending: {target_today_dollars: 80000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`, 2026)
	first := results[0]
	if first.Surplus <= 0 || first.UnallocatedSurplus != 0 {
		t.Fatalf("surplus/unallocated = %v/%v, want positive at the declared destination", first.Surplus, first.UnallocatedSurplus)
	}
	if got := first.Spouses[0].Surplus + first.Spouses[1].Surplus; got != first.Surplus {
		t.Fatalf("spouse surplus sums to %v, want the household surplus %v", got, first.Surplus)
	}
	taxable := accountYear(t, first, "Joint taxable")
	if want := RoundCents(200000*1.05) + first.Surplus; taxable.End != want {
		t.Fatalf("Joint taxable end = %v, want %v (the whole household surplus)", taxable.End, want)
	}
}

// TestRun_UnallocatedSurplusIsReported covers a working household with no
// taxable account: the config is valid, the projection completes, and the
// surplus that has nowhere to go is reported rather than silently dropped.
func TestRun_UnallocatedSurplusIsReported(t *testing.T) {
	results := mustRun(t, `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 100000}
  - {name: Sam, birth_year: 1983, retirement_age: 62}
accounts:
  - {name: Savings RRSP, type: rrsp, owner: Alex, balance: 500000}
spending: {target_today_dollars: 40000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`, 2026)
	first := results[0]
	if first.Surplus <= 0 {
		t.Fatalf("surplus = %v, want positive", first.Surplus)
	}
	if first.UnallocatedSurplus != first.Surplus {
		t.Fatalf("unallocated = %v, want the whole surplus %v with no destination", first.UnallocatedSurplus, first.Surplus)
	}
	if got := first.Spouses[0].UnallocatedSurplus; got != first.Surplus {
		t.Fatalf("Alex unallocated = %v, want %v", got, first.Surplus)
	}
	if want := RoundCents(500000 * 1.05); first.EndTotal != want {
		t.Fatalf("end total = %v, want only the RRSP growth %v", first.EndTotal, want)
	}
}

// TestRun_RetirementYearProratesCPPContributions covers the mid-year
// retirement approximation: the retirement year's contribution is computed on
// the half-year income with the basic exemption prorated to match.
func TestRun_RetirementYearProratesCPPContributions(t *testing.T) {
	results := mustRun(t, `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1966, retirement_age: 60, employment_income: 120000}
  - {name: Sam, birth_year: 1966, retirement_age: 65}
accounts:
  - {name: Savings, type: non_registered, owner: Alex, balance: 100000}
spending: {target_today_dollars: 40000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021, wage_growth: 0.031}
`, 2026)
	first := results[0]
	if got := first.EmploymentIncome; got != 60000 {
		t.Fatalf("employment income = %v, want the half-year 60000", got)
	}
	want := RoundCents(58250 * 0.0595)
	if math.Abs(first.CPPContributions-want) > 0.005 {
		t.Fatalf("CPP contributions = %v, want %v (half-year income, prorated exemption)", first.CPPContributions, want)
	}
}

// TestRun_CPPContributionsPrecedeTheWithdrawalSolve covers the cash-flow
// ordering: payroll contributions come off earnings before the solver decides
// whether the portfolio needs to top up the target. Here earnings would cover
// the target without the withholding, but the contribution opens a shortfall.
func TestRun_CPPContributionsPrecedeTheWithdrawalSolve(t *testing.T) {
	results := mustRun(t, `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 60, employment_income: 30000}
  - {name: Sam, birth_year: 1983, retirement_age: 62}
accounts:
  - {name: Savings, type: non_registered, owner: Alex, balance: 500000}
spending: {target_today_dollars: 28000, mode: flat}
assumptions: {portfolio_return: 0.055, inflation: 0.021}
`, 2026)
	first := results[0]
	if first.CPPContributions <= 0 {
		t.Fatalf("CPP contributions = %v, want withheld from the earnings", first.CPPContributions)
	}
	if first.Withdrawals <= 0 {
		t.Fatalf("withdrawals = %v, want a top-up once the contribution is withheld", first.Withdrawals)
	}
	if first.Surplus != 0 {
		t.Fatalf("surplus = %v, want 0 when the contribution opens a shortfall", first.Surplus)
	}
	if first.NetSpending != first.TargetNominal {
		t.Fatalf("net spending = %v, want the funded target %v", first.NetSpending, first.TargetNominal)
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
	// Mandatory income now also carries the couple's CPP and OAS; what
	// remains is the RRIF minimum this test is about.
	rrifMinimum2054 := year2054.MandatoryIncome - year2054.CPP - year2054.OAS
	if want := RoundCents(rrif2054.Begin * 0.0528); math.Abs(rrifMinimum2054-want) > 0.005 {
		t.Fatalf("2054 RRIF minimum = %v, want %v (age at Jan 1 is 71: 5.28%% of Jan-1 balance)", rrifMinimum2054, want)
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
	// Mandatory income also carries the survivor's OAS at this age; the RRIF
	// minimum is what remains.
	if minimum := year2074.MandatoryIncome - year2074.CPP - year2074.OAS; math.Abs(minimum-want2074) > 0.005 {
		t.Fatalf("2074 mandatory = %v, want RRIF minimum %v (Sam's would-be age 90)", minimum, want2074)
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

// TestRun_PerSpouseIncomeAndTaxBreakdown covers the Done-when item that each
// year carries income and tax separately for both spouses: unequal incomes
// stay split, and the spouse rows reconcile with the household totals.
func TestRun_PerSpouseIncomeAndTaxBreakdown(t *testing.T) {
	results := mustRun(t, baseHousehold, 2026)
	res := results[2046-2026]
	if len(res.Spouses) != 2 {
		t.Fatalf("2046 spouse rows = %d, want 2", len(res.Spouses))
	}
	alex, sam := res.Spouses[0], res.Spouses[1]
	if alex.Name != "Alex" || sam.Name != "Sam" {
		t.Fatalf("spouse names = %q/%q, want Alex/Sam", alex.Name, sam.Name)
	}
	if alex.GrossIncome <= sam.GrossIncome {
		t.Fatalf("Alex income %v should exceed Sam's %v (Alex owns the drawn accounts)", alex.GrossIncome, sam.GrossIncome)
	}
	if alex.TaxableIncome <= sam.TaxableIncome {
		t.Fatalf("Alex taxable income %v should exceed Sam's %v", alex.TaxableIncome, sam.TaxableIncome)
	}
	if alex.Tax <= sam.Tax {
		t.Fatalf("Alex tax %v should exceed Sam's %v on the unequal incomes", alex.Tax, sam.Tax)
	}
	if sum := alex.GrossIncome + sam.GrossIncome; math.Abs(sum-res.GrossIncome) > 0.02 {
		t.Fatalf("spouse incomes sum to %v, household gross income is %v", sum, res.GrossIncome)
	}
	if sum := alex.Tax + sam.Tax; math.Abs(sum-res.Tax) > 0.02 {
		t.Fatalf("spouse taxes sum to %v, household tax is %v", sum, res.Tax)
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

// withinDollar asserts the ±$1 tolerance the governing spec uses for
// published benefit figures.
func withinDollar(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1 {
		t.Fatalf("got %v, want %v within $1", got, want)
	}
}

// TestRun_CPPAndOASStartAtConfiguredAges covers the Done-when item that each
// spouse's CPP is paid from their chosen start age at the actuarial factor,
// and OAS defers to the chosen age: nothing before the start year, and both
// indexed in pay.
func TestRun_CPPAndOASStartAtConfiguredAges(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1961, retirement_age: 40, cpp: {monthly_at_65: 1000, start_age: 65}, oas: {start_age: 65}}
  - {name: Sam, birth_year: 1966, retirement_age: 40, cpp: {monthly_at_65: 1000, start_age: 70}, oas: {start_age: 70}}
spending: {target_today_dollars: 10000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	cpi10 := math.Pow(1.021, 10)

	// 2026: Alex is 65, so only Alex's CPP and OAS are paid. With no
	// accounts, they are the year's entire mandatory income.
	withinDollar(t, results[0].CPP, 12*1000)
	withinDollar(t, results[0].OAS, 12*742.31)
	withinDollar(t, results[0].MandatoryIncome, results[0].CPP+results[0].OAS)

	// 2035: Sam is 69, so Sam's CPP and OAS are still zero.
	withinDollar(t, results[2035-2026].CPP, 12*1000*math.Pow(1.021, 9))
	withinDollar(t, results[2035-2026].OAS, 12*742.31*math.Pow(1.021, 9))

	// 2036: Sam turns 70 — CPP at 1.420x and OAS at +36%, while Alex turns 75
	// into the permanent +10% OAS rate.
	withinDollar(t, results[2036-2026].CPP, 12*1000*cpi10+12*1000*1.42*cpi10)
	withinDollar(t, results[2036-2026].OAS, 12*816.54*cpi10+12*742.31*1.36*cpi10)
}

// TestRun_NoGISBeforeOASStarts covers the Done-when item that GIS is
// evaluated in every projection year: it is nil before the spouse receives
// OAS, then appears for a low-income pensioner.
func TestRun_NoGISBeforeOASStarts(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1966, retirement_age: 40, oas: {start_age: 65}}
  - {name: Sam, birth_year: 1968, retirement_age: 40, oas: {start_age: 65}}
spending: {target_today_dollars: 10000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	if results[0].OAS != 0 || results[0].GIS != 0 {
		t.Fatalf("2026 OAS/GIS = %v/%v, want 0 before any OAS start age", results[0].OAS, results[0].GIS)
	}
	if results[2031-2026].GIS <= 0 {
		t.Fatalf("2031 GIS = %v, want positive once OAS is received on a low income", results[2031-2026].GIS)
	}
}

// TestRun_OASRecoveryClawsBackAtHighIncome covers the Done-when item that the
// 15% recovery tax is applied over the threshold and reaches full clawback:
// a high-income household's OAS is recovered in full, and the gross-vs-net
// solve compensates for the recovery.
func TestRun_OASRecoveryClawsBackAtHighIncome(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1956, retirement_age: 40}
  - {name: Sam, birth_year: 1956, retirement_age: 40}
accounts:
  - {name: Alex RRIF, type: rrif, owner: Alex, balance: 4000000}
  - {name: Sam RRIF, type: rrif, owner: Sam, balance: 4000000}
spending: {target_today_dollars: 300000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	first := mustRun(t, yamlText, 2026)[0]
	if first.OAS <= 0 {
		t.Fatalf("OAS = %v, want a positive pension before the recovery", first.OAS)
	}
	withinDollar(t, first.OASRecovery, first.OAS)
	if first.GIS != 0 {
		t.Fatalf("GIS = %v, want 0 for a high-income household", first.GIS)
	}
	if math.Abs(first.NetSpending-first.TargetNominal) > 1 {
		t.Fatalf("net spending = %v, want the funded target %v after the recovery", first.NetSpending, first.TargetNominal)
	}
}

// TestRun_GISIsZeroForAffluentHousehold covers the zero years of the
// Done-when item: an affluent household is evaluated every year and never
// qualifies.
func TestRun_GISIsZeroForAffluentHousehold(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1956, retirement_age: 40, cpp: {monthly_at_65: 1500, start_age: 65}}
  - {name: Sam, birth_year: 1958, retirement_age: 40, cpp: {monthly_at_65: 1200, start_age: 65}}
accounts:
  - {name: Alex RRIF, type: rrif, owner: Alex, balance: 5000000}
  - {name: Sam RRIF, type: rrif, owner: Sam, balance: 3000000}
spending: {target_today_dollars: 120000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	for _, res := range mustRun(t, yamlText, 2026) {
		if res.GIS != 0 {
			t.Fatalf("GIS in %d = %v, want 0 for an affluent household", res.Year, res.GIS)
		}
		if res.OAS <= 0 {
			t.Fatalf("OAS in %d = %v, want the pension evaluated every year", res.Year, res.OAS)
		}
	}
}

// TestRun_GISReappearsForLowIncomeSurvivor covers the reason GIS is evaluated
// every year: a low-income survivor brings it back above zero.
func TestRun_GISReappearsForLowIncomeSurvivor(t *testing.T) {
	yamlText := `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1956, retirement_age: 60, death_age: 75, cpp: {monthly_at_65: 200, start_age: 65}}
  - {name: Sam, birth_year: 1956, retirement_age: 60, death_age: 95, cpp: {monthly_at_65: 200, start_age: 65}}
spending: {target_today_dollars: 10000, mode: flat}
assumptions: {portfolio_return: 0.03, inflation: 0.021}
`
	results := mustRun(t, yamlText, 2026)
	if got := results[2031-2026].Deaths; len(got) != 1 || got[0] != "Alex" {
		t.Fatalf("2031 deaths = %v, want [Alex]", got)
	}
	if results[2032-2026].GIS <= 0 {
		t.Fatalf("2032 GIS = %v, want positive for a low-income survivor", results[2032-2026].GIS)
	}
}

// TestRun_CPPSharingShiftsIncomeBetweenSpouses covers the Done-when item that
// a user-set sharing election is honoured without changing the combined
// pensions, and that the default of zero leaves them untouched.
func TestRun_CPPSharingShiftsIncomeBetweenSpouses(t *testing.T) {
	const shared = `base_year: 2026
cpp_sharing_fraction: 0.5
spouses:
  - {name: Alex, birth_year: 1956, retirement_age: 40, cpp: {monthly_at_65: 2000, start_age: 65}}
  - {name: Sam, birth_year: 1956, retirement_age: 40, cpp: {monthly_at_65: 0, start_age: 65}}
accounts:
  - {name: Alex RRIF, type: rrif, owner: Alex, balance: 800000}
  - {name: Sam RRIF, type: rrif, owner: Sam, balance: 200000}
spending: {target_today_dollars: 100000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	withSharing := mustRun(t, shared, 2026)

	h := mustHousehold(t, shared)
	h.CPPSharingFraction = 0
	withoutSharing, err := Run(h, 2026)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	withinDollar(t, withSharing[0].CPP, withoutSharing[0].CPP)
	if withSharing[0].Tax >= withoutSharing[0].Tax {
		t.Fatalf("shared tax %v, want below the unshared tax %v (income shifted to the lower earner)",
			withSharing[0].Tax, withoutSharing[0].Tax)
	}
	if math.Abs(withSharing[0].NetSpending-withSharing[0].TargetNominal) > 1 {
		t.Fatalf("shared net spending = %v, want the funded target %v", withSharing[0].NetSpending, withSharing[0].TargetNominal)
	}
}

// TestApplyCPPSharing_OnlyWhileBothReceive covers the election's guard: the
// pooling applies only while both spouses are alive and past their own CPP
// start age.
func TestApplyCPPSharing_OnlyWhileBothReceive(t *testing.T) {
	s := &State{Year: 2026, People: []Person{
		{Name: "Alex", BirthYear: 1961, CPPStartAge: 65, Alive: true},
		{Name: "Sam", BirthYear: 1966, CPPStartAge: 70, Alive: true},
	}}
	h := &config.Household{CPPSharingFraction: 1}

	cpp := []float64{12000, 0}
	if err := s.applyCPPSharing(h, cpp); err != nil {
		t.Fatalf("applyCPPSharing: %v", err)
	}
	if cpp[0] != 12000 || cpp[1] != 0 {
		t.Fatalf("pensions = %v/%v, want 12000/0 before both receive CPP", cpp[0], cpp[1])
	}

	s.Year = 2036
	if err := s.applyCPPSharing(h, cpp); err != nil {
		t.Fatalf("applyCPPSharing: %v", err)
	}
	if cpp[0] != 6000 || cpp[1] != 6000 {
		t.Fatalf("pensions = %v/%v, want the pooled 6000/6000 once both receive", cpp[0], cpp[1])
	}

	s.People[1].Alive = false
	cpp = []float64{12000, 0}
	if err := s.applyCPPSharing(h, cpp); err != nil {
		t.Fatalf("applyCPPSharing: %v", err)
	}
	if cpp[0] != 12000 || cpp[1] != 0 {
		t.Fatalf("pensions = %v/%v, want no sharing after a death", cpp[0], cpp[1])
	}
}
