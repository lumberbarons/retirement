package report

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"text/tabwriter"

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

func (r *Report) WriteCSV(w io.Writer) error {
	if len(r.Years) == 0 {
		return errEmpty
	}
	cw := csv.NewWriter(w)
	header := []string{
		"year",
		"mandatory_income_nominal", "mandatory_income_real",
		"withdrawals_nominal", "withdrawals_real",
		"gross_income_nominal", "gross_income_real",
		"tax_nominal", "tax_real",
		"net_spending_nominal", "net_spending_real",
		"end_balance_nominal", "end_balance_real",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, y := range r.Years {
		row := []string{
			strconv.Itoa(y.Year),
			cents(y.MandatoryIncome), cents(r.Real(y.MandatoryIncome, y.Year)),
			cents(y.Withdrawals), cents(r.Real(y.Withdrawals, y.Year)),
			cents(y.GrossIncome), cents(r.Real(y.GrossIncome, y.Year)),
			cents(y.Tax), cents(r.Real(y.Tax, y.Year)),
			cents(y.NetSpending), cents(r.Real(y.NetSpending, y.Year)),
			cents(y.EndTotal), cents(r.Real(y.EndTotal, y.Year)),
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
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "year\tspend nominal\tspend today's\tincome nominal\ttax nominal\twithdrawals nominal\tend balance nominal\tend balance today's")
	for _, y := range r.Years {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			y.Year,
			cents(y.NetSpending), cents(r.Real(y.NetSpending, y.Year)),
			cents(y.GrossIncome),
			cents(y.Tax),
			cents(y.Withdrawals),
			cents(y.EndTotal), cents(r.Real(y.EndTotal, y.Year)),
		)
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
