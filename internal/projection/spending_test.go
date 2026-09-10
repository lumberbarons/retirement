package projection

import (
	"math"
	"strings"
	"testing"

	"github.com/lumberbarons/retirement/internal/config"
)

const spendingHouseholdYAML = `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 40}
  - {name: Sam, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: Fund, type: non_registered, owner: Alex, balance: 5000000}
spending:
  target_today_dollars: 80000
  mode: flat
  inflation: 0.02
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`

func spendingState(t *testing.T, yamlText string, year int) (*config.Household, *State) {
	t.Helper()
	h := mustHousehold(t, yamlText)
	s := NewState(h, 2026)
	s.Year = year
	return h, s
}

func TestNominalSpending_UsesSpendingInflationNotTaxIndexation(t *testing.T) {
	h, s := spendingState(t, spendingHouseholdYAML, 2036)
	want := 80000 * math.Pow(1.02, 10)
	if got := s.nominalSpending(h); math.Abs(got-want) > 0.01 {
		t.Fatalf("2036 target = %v, want %v (spending inflation 2%%)", got, want)
	}
	if cpi := 80000 * math.Pow(1.021, 10); math.Abs(want-cpi) < 1 {
		t.Fatalf("test household must divorce spending inflation from CPI, got %v vs %v", want, cpi)
	}
}

func TestNominalSpending_FlatModeIsFlatInRealTerms(t *testing.T) {
	for _, year := range []int{2026, 2041, 2071} {
		h, s := spendingState(t, spendingHouseholdYAML, year)
		want := 80000 * math.Pow(1.02, float64(year-2026))
		if got := s.nominalSpending(h); math.Abs(got-want) > 0.01 {
			t.Fatalf("year %d target = %v, want %v (flat real)", year, got, want)
		}
	}
}

func TestSmileFactor_GoGoThenOnePercentDeclineThenPlateau(t *testing.T) {
	plateau := math.Pow(0.99, 15)
	cases := []struct {
		years int
		want  float64
	}{
		{0, 1},
		{9, 1},
		{10, 0.99},
		{24, plateau},
		{25, plateau},
		{40, plateau},
	}
	for _, tc := range cases {
		if got := smileFactor(tc.years); math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("smileFactor(%d) = %v, want %v", tc.years, got, tc.want)
		}
	}
}

func TestNominalSpending_SmileModeDeclinesMidPhase(t *testing.T) {
	smileYAML := strings.Replace(spendingHouseholdYAML, "mode: flat", "mode: smile", 1)

	// Retirement started in 2021 (birth year + retirement age), so 2026 is
	// inside the flat go-go phase and 2031 is the first declining year.
	h, s := spendingState(t, smileYAML, 2026)
	if got, want := s.nominalSpending(h), 80000.0; math.Abs(got-want) > 0.01 {
		t.Fatalf("go-go year target = %v, want %v", got, want)
	}

	h, s = spendingState(t, smileYAML, 2031)
	want := 80000 * 0.99 * math.Pow(1.02, 5)
	if got := s.nominalSpending(h); math.Abs(got-want) > 0.01 {
		t.Fatalf("2031 target = %v, want %v (one year of the 1%% real decline)", got, want)
	}

	h, s = spendingState(t, smileYAML, 2046)
	want = 80000 * math.Pow(0.99, 15) * math.Pow(1.02, 20)
	if got := s.nominalSpending(h); math.Abs(got-want) > 0.01 {
		t.Fatalf("2046 target = %v, want %v (decline has plateaued at 0.99^15)", got, want)
	}
}

func TestNominalSpending_LumpyStreamsAddInTheirYears(t *testing.T) {
	const yamlText = `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 40}
  - {name: Sam, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: Fund, type: non_registered, owner: Alex, balance: 5000000}
spending:
  target_today_dollars: 80000
  mode: flat
  inflation: 0.02
  lumpy:
    - {name: Travel, amount_today_dollars: 5000, start_year: 2027, end_year: 2028}
    - {name: Roof, amount_today_dollars: 20000, start_year: 2028}
    - {name: Car, amount_today_dollars: 30000, start_year: 2027, end_year: 2043, every_years: 8}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	cases := []struct {
		year int
		want float64
	}{
		{2026, 80000},
		{2027, 80000 + 5000 + 30000},
		{2028, 80000 + 5000 + 20000},
		{2029, 80000},
		{2035, 80000 + 30000},
		{2043, 80000 + 30000},
		{2044, 80000},
	}
	for _, tc := range cases {
		h, s := spendingState(t, yamlText, tc.year)
		want := tc.want * math.Pow(1.02, float64(tc.year-2026))
		if got := s.nominalSpending(h); math.Abs(got-want) > 0.01 {
			t.Errorf("year %d target = %v, want %v", tc.year, got, want)
		}
	}
}

func TestNominalSpending_SurvivorFactorAppliesAfterFirstDeath(t *testing.T) {
	const yamlText = `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1981, retirement_age: 40}
  - {name: Sam, birth_year: 1983, retirement_age: 40}
accounts:
  - {name: Fund, type: non_registered, owner: Alex, balance: 5000000}
spending:
  target_today_dollars: 80000
  mode: flat
  inflation: 0.02
  survivor_factor: 0.70
  lumpy:
    - {name: Care, amount_today_dollars: 10000, start_year: 2036}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	h, s := spendingState(t, yamlText, 2036)
	wantCouple := (80000 + 10000) * math.Pow(1.02, 10)
	if got := s.nominalSpending(h); math.Abs(got-wantCouple) > 0.01 {
		t.Fatalf("couple target = %v, want %v", got, wantCouple)
	}

	s.People[0].Alive = false
	wantSurvivor := (80000*0.70 + 10000) * math.Pow(1.02, 10)
	if got := s.nominalSpending(h); math.Abs(got-wantSurvivor) > 0.01 {
		t.Fatalf("survivor target = %v, want %v (lifestyle at 70%%, dated stream whole)", got, wantSurvivor)
	}
}
