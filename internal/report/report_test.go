package report

import (
	"encoding/csv"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/lumberbarons/retirement/internal/config"
	"github.com/lumberbarons/retirement/internal/projection"
)

func money(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func testYears() []projection.YearResult {
	return []projection.YearResult{
		{Year: 2046, BeginTotal: 2000000, EndTotal: 1900000, NetSpending: 120982, GrossIncome: 121000, Tax: 0, Withdrawals: 121000, MandatoryIncome: 0},
		{Year: 2047, BeginTotal: 1900000, EndTotal: 1790000, NetSpending: 123520, GrossIncome: 124000, Tax: 0, Withdrawals: 124000, MandatoryIncome: 0},
	}
}

func TestReal_DeflatesByCPISinceBaseYear(t *testing.T) {
	r := New(2026, 0.021, testYears())
	want := 121000 / math.Pow(1.021, 20)
	if math.Abs(r.Real(121000, 2046)-want) > 0.01 {
		t.Fatalf("Real(121000, 2046) = %v, want %v", r.Real(121000, 2046), want)
	}
	if math.Abs(r.Real(100, 2026)-100) > 0.000001 {
		t.Fatalf("Real at base year = %v, want 100", r.Real(100, 2026))
	}
}

func TestWriteCSV_ShowsEveryYearInBothFrames(t *testing.T) {
	r := New(2026, 0.021, testYears())
	var buf strings.Builder
	if err := r.WriteCSV(&buf); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	rows, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("csv rows = %d, want 3 (header + 2 years)", len(rows))
	}
	header := strings.Join(rows[0], ",")
	for _, col := range []string{"year", "net_spending_nominal", "net_spending_real", "end_balance_nominal", "end_balance_real"} {
		if !strings.Contains(header, col) {
			t.Fatalf("csv header %q missing column %q", header, col)
		}
	}
	for i, res := range testYears() {
		row := rows[i+1]
		if row[0] != strconv.Itoa(res.Year) {
			t.Fatalf("row %d year = %s, want %d", i+1, row[0], res.Year)
		}
		nominal := row[len(row)-2]
		realCol := row[len(row)-1]
		if nominal != money(res.EndTotal) {
			t.Fatalf("row %d end nominal = %s, want %v", i+1, nominal, res.EndTotal)
		}
		wantReal := r.Real(res.EndTotal, res.Year)
		gotReal, err := strconv.ParseFloat(realCol, 64)
		if err != nil || math.Abs(gotReal-wantReal) > 0.01 {
			t.Fatalf("row %d end real = %s, want ~%v", i+1, realCol, wantReal)
		}
	}
}

func TestWriteTable_ShowsEveryYearInBothFrames(t *testing.T) {
	r := New(2026, 0.021, testYears())
	var buf strings.Builder
	if err := r.WriteTable(&buf); err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"2046", "2047", "nominal", "today"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table missing %q:\n%s", want, out)
		}
	}
}

// TestWriteCSV_ShowsGovernmentBenefitsAndClawback covers the issue's goal
// that the projection shows what the household receives and what gets clawed
// back: CPP, OAS, GIS, and the recovery tax each get a column, in both frames.
func TestWriteCSV_ShowsGovernmentBenefitsAndClawback(t *testing.T) {
	years := []projection.YearResult{
		{Year: 2046, CPP: 12000, OAS: 8907.72, OASRecovery: 8907.72, GIS: 1234.56},
	}
	r := New(2026, 0.021, years)
	var buf strings.Builder
	if err := r.WriteCSV(&buf); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	rows, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("csv rows = %d, want 2 (header + 1 year)", len(rows))
	}
	column := map[string]int{}
	for i, name := range rows[0] {
		column[name] = i
	}
	for name, want := range map[string]float64{
		"cpp_nominal":          12000,
		"cpp_real":             r.Real(12000, 2046),
		"oas_nominal":          8907.72,
		"oas_real":             r.Real(8907.72, 2046),
		"oas_recovery_nominal": 8907.72,
		"oas_recovery_real":    r.Real(8907.72, 2046),
		"gis_nominal":          1234.56,
		"gis_real":             r.Real(1234.56, 2046),
	} {
		idx, ok := column[name]
		if !ok {
			t.Fatalf("csv header missing column %q", name)
		}
		got, err := strconv.ParseFloat(rows[1][idx], 64)
		if err != nil || math.Abs(got-want) > 0.01 {
			t.Fatalf("column %s = %s, want ~%v", name, rows[1][idx], want)
		}
	}

	var table strings.Builder
	if err := r.WriteTable(&table); err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	for _, want := range []string{"cpp nominal", "oas nominal", "oas recovery nominal", "gis nominal"} {
		if !strings.Contains(table.String(), want) {
			t.Fatalf("table missing %q:\n%s", want, table.String())
		}
	}
}

// TestWriteCSV_ShowsCPPContributionsAndRetainedSurplus covers the working-year
// cash flow: payroll contributions and any retained or unallocated surplus get
// columns in both frames.
func TestWriteCSV_ShowsCPPContributionsAndRetainedSurplus(t *testing.T) {
	years := []projection.YearResult{
		{Year: 2026, CPPContributions: 4646.45, Surplus: 20000, UnallocatedSurplus: 1500},
	}
	r := New(2026, 0.021, years)
	row := csvYearRow(t, r, 2026)
	for name, want := range map[string]float64{
		"cpp_contributions_nominal":   4646.45,
		"cpp_contributions_real":      4646.45,
		"surplus_nominal":             20000,
		"surplus_real":                20000,
		"unallocated_surplus_nominal": 1500,
		"unallocated_surplus_real":    1500,
	} {
		if got := columnFloat(t, row, name); math.Abs(got-want) > 0.01 {
			t.Fatalf("column %s = %v, want ~%v", name, got, want)
		}
	}
	var table strings.Builder
	if err := r.WriteTable(&table); err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	for _, want := range []string{"cpp contributions nominal", "surplus nominal", "unallocated surplus nominal"} {
		if !strings.Contains(table.String(), want) {
			t.Fatalf("table missing %q:\n%s", want, table.String())
		}
	}
}

func TestSummary_ReadsTerminalRealWealthAgainstZeroEstateTarget(t *testing.T) {
	r := New(2026, 0.021, testYears())
	s := r.Summary()
	if s.EstateTarget != 0 {
		t.Fatalf("estate target = %v, want 0 (zero-bequest household)", s.EstateTarget)
	}
	if s.TerminalYear != 2047 {
		t.Fatalf("terminal year = %d, want 2047", s.TerminalYear)
	}
	wantReal := r.Real(1790000, 2047)
	if math.Abs(s.TerminalRealWealth-wantReal) > 0.01 {
		t.Fatalf("terminal real wealth = %v, want %v", s.TerminalRealWealth, wantReal)
	}
	if s.OnTarget {
		t.Fatal("a large residual should not read as on target")
	}
	if s.Residual <= 0 {
		t.Fatalf("residual = %v, want positive", s.Residual)
	}
	var buf strings.Builder
	if err := r.WriteSummary(&buf); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "miss") {
		t.Fatalf("summary should frame a large residual as a miss:\n%s", out)
	}
}

func TestSummary_OnTargetWhenResidualIsTrivial(t *testing.T) {
	years := []projection.YearResult{
		{Year: 2030, EndTotal: 0.5},
	}
	r := New(2026, 0.021, years)
	s := r.Summary()
	if !s.OnTarget {
		t.Fatalf("residual %v within $1 should read as on target", s.Residual)
	}
	var buf strings.Builder
	if err := r.WriteSummary(&buf); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}
	if !strings.Contains(buf.String(), "on target") {
		t.Fatalf("summary should say on target:\n%s", buf.String())
	}
}

func TestWriteCSV_EmptyProjectionErrors(t *testing.T) {
	r := New(2026, 0.021, nil)
	if err := r.WriteCSV(io.Discard); err == nil {
		t.Fatal("WriteCSV on an empty projection should error")
	}
}

// spouseAndAccountYears is a one-year fixture with unequal spouse incomes and
// withdrawals from two different accounts.
func spouseAndAccountYears() []projection.YearResult {
	return []projection.YearResult{
		{
			Year: 2046, BeginTotal: 700000, EndTotal: 660000,
			Spouses: []projection.SpouseYear{
				{Name: "Alex", GrossIncome: 90000, TaxableIncome: 85000, Tax: 18000},
				{Name: "Sam", GrossIncome: 30000, TaxableIncome: 25000, Tax: 2500},
			},
			Accounts: []projection.AccountYear{
				{Name: "Alex RRSP", Begin: 450000, Withdrawal: 60000, End: 412500},
				{Name: "Sam TFSA", Begin: 100000, Withdrawal: 20000, End: 85000},
			},
		},
	}
}

// csvYearRow parses a report and returns the data row for the given year,
// keyed by header name.
func csvYearRow(t *testing.T, r *Report, year int) map[string]string {
	t.Helper()
	var buf strings.Builder
	if err := r.WriteCSV(&buf); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	rows, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	out := map[string]string{}
	for i, res := range r.Years {
		if res.Year != year {
			continue
		}
		row := rows[i+1]
		for j, name := range rows[0] {
			out[name] = row[j]
		}
		return out
	}
	t.Fatalf("csv has no row for year %d", year)
	return nil
}

func columnFloat(t *testing.T, row map[string]string, name string) float64 {
	t.Helper()
	raw, ok := row[name]
	if !ok {
		t.Fatalf("csv missing column %q (have %v)", name, row)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("column %s = %q is not a number", name, raw)
	}
	return v
}

// TestWriteCSV_ShowsPerSpouseIncomeAndTax covers the Done-when item that each
// year reports income and tax separately for both spouses.
func TestWriteCSV_ShowsPerSpouseIncomeAndTax(t *testing.T) {
	r := New(2026, 0.021, spouseAndAccountYears())
	row := csvYearRow(t, r, 2046)
	for name, want := range map[string]float64{
		"alex_income_nominal":         90000,
		"alex_income_real":            90000 / math.Pow(1.021, 20),
		"alex_taxable_income_nominal": 85000,
		"alex_tax_nominal":            18000,
		"alex_tax_real":               18000 / math.Pow(1.021, 20),
		"sam_income_nominal":          30000,
		"sam_income_real":             30000 / math.Pow(1.021, 20),
		"sam_taxable_income_nominal":  25000,
		"sam_tax_nominal":             2500,
	} {
		if got := columnFloat(t, row, name); math.Abs(got-want) > 0.01 {
			t.Fatalf("column %s = %v, want ~%v", name, got, want)
		}
	}
}

// TestWriteCSV_ShowsAccountBalancesAndWithdrawals covers the Done-when item
// that each year reports beginning balance, withdrawals, and ending balance
// for every account.
func TestWriteCSV_ShowsAccountBalancesAndWithdrawals(t *testing.T) {
	r := New(2026, 0.021, spouseAndAccountYears())
	row := csvYearRow(t, r, 2046)
	for name, want := range map[string]float64{
		"alex_rrsp_begin_nominal":       450000,
		"alex_rrsp_begin_real":          450000 / math.Pow(1.021, 20),
		"alex_rrsp_withdrawals_nominal": 60000,
		"alex_rrsp_end_nominal":         412500,
		"alex_rrsp_end_real":            412500 / math.Pow(1.021, 20),
		"sam_tfsa_begin_nominal":        100000,
		"sam_tfsa_withdrawals_nominal":  20000,
		"sam_tfsa_withdrawals_real":     20000 / math.Pow(1.021, 20),
		"sam_tfsa_end_nominal":          85000,
	} {
		if got := columnFloat(t, row, name); math.Abs(got-want) > 0.01 {
			t.Fatalf("column %s = %v, want ~%v", name, got, want)
		}
	}
}

// TestWriteTable_ShowsSpouseAndAccountDetail covers the Done-when item that
// the text output exposes the same spouse and account detail as the CSV,
// alongside the retained household totals.
func TestWriteTable_ShowsSpouseAndAccountDetail(t *testing.T) {
	r := New(2026, 0.021, spouseAndAccountYears())
	var buf strings.Builder
	if err := r.WriteTable(&buf); err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"end balance nominal",
		"Alex", "Sam",
		"income nominal", "tax nominal", "taxable income nominal",
		"Alex RRSP", "Sam TFSA",
		"begin nominal", "withdrawals nominal", "end nominal",
		"90000.00", "18000.00",
		"450000.00", "60000.00", "412500.00",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("table missing %q:\n%s", want, out)
		}
	}
}

// TestWriteCSV_ProjectionShowsUnequalSpouseIncomesAndMultipleWithdrawals
// covers the Done-when item that report tests verify unequal spouse incomes
// and withdrawals from more than one account: it runs a real projection and
// checks the rendered CSV, not a hand-built fixture.
func TestWriteCSV_ProjectionShowsUnequalSpouseIncomesAndMultipleWithdrawals(t *testing.T) {
	const household = `base_year: 2026
spouses:
  - {name: Alex, birth_year: 1956, retirement_age: 40}
  - {name: Sam, birth_year: 1958, retirement_age: 40}
accounts:
  - {name: Alex RRSP, type: rrsp, owner: Alex, balance: 700000}
  - {name: Sam TFSA, type: tfsa, owner: Sam, balance: 120000}
  - {name: Joint taxable, type: non_registered, owner: Alex, balance: 50000, acb: 40000}
spending: {target_today_dollars: 90000, mode: flat}
assumptions: {portfolio_return: 0.05, inflation: 0.021}
`
	h, err := config.Parse([]byte(household))
	if err != nil {
		t.Fatalf("config parse: %v", err)
	}
	years, err := projection.Run(h, 2026)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := New(h.BaseYear, h.Assumptions.Inflation, years)
	row := csvYearRow(t, r, 2026)

	alexIncome := columnFloat(t, row, "alex_income_nominal")
	samIncome := columnFloat(t, row, "sam_income_nominal")
	if alexIncome <= samIncome {
		t.Fatalf("Alex income %v should exceed Sam's %v (Alex owns the drawn registered accounts)", alexIncome, samIncome)
	}

	drawn := 0
	for _, account := range []string{"joint_taxable", "alex_rrsp", "sam_tfsa"} {
		if columnFloat(t, row, account+"_withdrawals_nominal") > 0 {
			drawn++
		}
	}
	if drawn < 2 {
		t.Fatalf("withdrawals from %d accounts, want more than one", drawn)
	}
}
