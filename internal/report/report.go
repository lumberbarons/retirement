package report

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/lumberbarons/retirement/internal/projection"
)

const estateTarget = 0

var errEmpty = errors.New("report: empty projection")

type Report struct {
	BaseYear int
	CPI      float64
	Years    []projection.YearResult
}

func New(baseYear int, cpi float64, years []projection.YearResult) *Report {
	return &Report{BaseYear: baseYear, CPI: cpi, Years: years}
}

func (r *Report) Real(nominal float64, year int) float64 {
	return nominal / math.Pow(1+r.CPI, float64(year-r.BaseYear))
}

type Summary struct {
	TerminalYear       int
	TerminalRealWealth float64
	EstateTarget       float64
	Residual           float64
	OnTarget           bool
}

func (r *Report) Summary() Summary {
	s := Summary{EstateTarget: estateTarget}
	if len(r.Years) == 0 {
		return s
	}
	last := r.Years[len(r.Years)-1]
	s.TerminalYear = last.Year
	s.TerminalRealWealth = r.Real(last.EndTotal, last.Year)
	s.Residual = s.TerminalRealWealth - s.EstateTarget
	s.OnTarget = math.Abs(s.Residual) <= 1
	return s
}

func cents(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// slug turns a spouse or account name into a CSV column fragment: lowercased,
// with runs of other characters collapsed to a single underscore.
func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
				b.WriteByte('_')
			}
		}
	}
	return strings.TrimRight(b.String(), "_")
}

// spouseNames returns each spouse once, in the order first seen, so the CSV
// columns and the table sections stay stable across years.
func (r *Report) spouseNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, y := range r.Years {
		for _, s := range y.Spouses {
			if !seen[s.Name] {
				seen[s.Name] = true
				names = append(names, s.Name)
			}
		}
	}
	return names
}

// accountNames returns each account once, in the order first seen.
func (r *Report) accountNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, y := range r.Years {
		for _, a := range y.Accounts {
			if !seen[a.Name] {
				seen[a.Name] = true
				names = append(names, a.Name)
			}
		}
	}
	return names
}

func spouseIn(y projection.YearResult, name string) projection.SpouseYear {
	for _, s := range y.Spouses {
		if s.Name == name {
			return s
		}
	}
	return projection.SpouseYear{}
}

func accountIn(y projection.YearResult, name string) projection.AccountYear {
	for _, a := range y.Accounts {
		if a.Name == name {
			return a
		}
	}
	return projection.AccountYear{}
}

func (r *Report) WriteCSV(w io.Writer) error {
	if len(r.Years) == 0 {
		return errEmpty
	}
	spouses := r.spouseNames()
	accounts := r.accountNames()
	cw := csv.NewWriter(w)
	header := []string{
		"year",
		"mandatory_income_nominal", "mandatory_income_real",
		"cpp_nominal", "cpp_real",
		"oas_nominal", "oas_real",
		"gis_nominal", "gis_real",
		"withdrawals_nominal", "withdrawals_real",
		"gross_income_nominal", "gross_income_real",
		"tax_nominal", "tax_real",
		"cpp_contributions_nominal", "cpp_contributions_real",
		"oas_recovery_nominal", "oas_recovery_real",
		"net_spending_nominal", "net_spending_real",
		"surplus_nominal", "surplus_real",
		"unallocated_surplus_nominal", "unallocated_surplus_real",
		"end_balance_nominal", "end_balance_real",
	}
	for _, name := range spouses {
		s := slug(name)
		header = append(header,
			s+"_income_nominal", s+"_income_real",
			s+"_taxable_income_nominal", s+"_taxable_income_real",
			s+"_tax_nominal", s+"_tax_real")
	}
	for _, name := range accounts {
		a := slug(name)
		header = append(header,
			a+"_begin_nominal", a+"_begin_real",
			a+"_withdrawals_nominal", a+"_withdrawals_real",
			a+"_end_nominal", a+"_end_real")
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, y := range r.Years {
		row := []string{
			strconv.Itoa(y.Year),
			cents(y.MandatoryIncome), cents(r.Real(y.MandatoryIncome, y.Year)),
			cents(y.CPP), cents(r.Real(y.CPP, y.Year)),
			cents(y.OAS), cents(r.Real(y.OAS, y.Year)),
			cents(y.GIS), cents(r.Real(y.GIS, y.Year)),
			cents(y.Withdrawals), cents(r.Real(y.Withdrawals, y.Year)),
			cents(y.GrossIncome), cents(r.Real(y.GrossIncome, y.Year)),
			cents(y.Tax), cents(r.Real(y.Tax, y.Year)),
			cents(y.CPPContributions), cents(r.Real(y.CPPContributions, y.Year)),
			cents(y.OASRecovery), cents(r.Real(y.OASRecovery, y.Year)),
			cents(y.NetSpending), cents(r.Real(y.NetSpending, y.Year)),
			cents(y.Surplus), cents(r.Real(y.Surplus, y.Year)),
			cents(y.UnallocatedSurplus), cents(r.Real(y.UnallocatedSurplus, y.Year)),
			cents(y.EndTotal), cents(r.Real(y.EndTotal, y.Year)),
		}
		for _, name := range spouses {
			s := spouseIn(y, name)
			row = append(row,
				cents(s.GrossIncome), cents(r.Real(s.GrossIncome, y.Year)),
				cents(s.TaxableIncome), cents(r.Real(s.TaxableIncome, y.Year)),
				cents(s.Tax), cents(r.Real(s.Tax, y.Year)))
		}
		for _, name := range accounts {
			a := accountIn(y, name)
			row = append(row,
				cents(a.Begin), cents(r.Real(a.Begin, y.Year)),
				cents(a.Withdrawal), cents(r.Real(a.Withdrawal, y.Year)),
				cents(a.End), cents(r.Real(a.End, y.Year)))
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func (r *Report) WriteTable(w io.Writer) error {
	if len(r.Years) == 0 {
		return errEmpty
	}
	if err := r.writeHouseholdTable(w); err != nil {
		return err
	}
	if err := r.writeSpouseTable(w); err != nil {
		return err
	}
	return r.writeAccountTable(w)
}

func (r *Report) writeHouseholdTable(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "year\tspend nominal\tspend today's\tincome nominal\tcpp nominal\toas nominal\toas recovery nominal\tgis nominal\ttax nominal\tcpp contributions nominal\twithdrawals nominal\tsurplus nominal\tunallocated surplus nominal\tend balance nominal\tend balance today's")
	for _, y := range r.Years {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			y.Year,
			cents(y.NetSpending), cents(r.Real(y.NetSpending, y.Year)),
			cents(y.GrossIncome),
			cents(y.CPP), cents(y.OAS), cents(y.OASRecovery), cents(y.GIS),
			cents(y.Tax),
			cents(y.CPPContributions),
			cents(y.Withdrawals),
			cents(y.Surplus),
			cents(y.UnallocatedSurplus),
			cents(y.EndTotal), cents(r.Real(y.EndTotal, y.Year)),
		)
	}
	return tw.Flush()
}

func (r *Report) writeSpouseTable(w io.Writer) error {
	if len(r.spouseNames()) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "")
	fmt.Fprintln(tw, "year\tspouse\tincome nominal\tincome today's\ttaxable income nominal\ttaxable income today's\ttax nominal\ttax today's")
	for _, y := range r.Years {
		for _, s := range y.Spouses {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				y.Year, s.Name,
				cents(s.GrossIncome), cents(r.Real(s.GrossIncome, y.Year)),
				cents(s.TaxableIncome), cents(r.Real(s.TaxableIncome, y.Year)),
				cents(s.Tax), cents(r.Real(s.Tax, y.Year)))
		}
	}
	return tw.Flush()
}

func (r *Report) writeAccountTable(w io.Writer) error {
	if len(r.accountNames()) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "")
	fmt.Fprintln(tw, "year\taccount\tbegin nominal\tbegin today's\twithdrawals nominal\twithdrawals today's\tend nominal\tend today's")
	for _, y := range r.Years {
		for _, a := range y.Accounts {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				y.Year, a.Name,
				cents(a.Begin), cents(r.Real(a.Begin, y.Year)),
				cents(a.Withdrawal), cents(r.Real(a.Withdrawal, y.Year)),
				cents(a.End), cents(r.Real(a.End, y.Year)))
		}
	}
	return tw.Flush()
}

func (r *Report) WriteSummary(w io.Writer) error {
	s := r.Summary()
	fmt.Fprintf(w, "terminal real wealth at second death (%d): $%s, against the $%s estate target",
		s.TerminalYear, cents(s.TerminalRealWealth), cents(s.EstateTarget))
	switch {
	case s.OnTarget:
		fmt.Fprintln(w, " — on target")
	case s.Residual > 0:
		fmt.Fprintln(w, " — a large residual is a miss, not a windfall")
	default:
		fmt.Fprintln(w, " — shortfall: the plan overspent")
	}
	return nil
}
