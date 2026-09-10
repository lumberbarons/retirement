package report

import (
	"encoding/csv"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

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
