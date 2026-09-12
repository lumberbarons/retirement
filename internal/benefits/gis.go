package benefits

import (
	"math"

	"github.com/lumberbarons/retirement/internal/constants"
)

// gisReductionRate is the rate at which GIS is reduced by other income.
const gisReductionRate = 0.50

// GISInput is one spouse-year's Guaranteed Income Supplement test.
type GISInput struct {
	Year    int
	Forward constants.Forward
	// Single applies the single rate and cut-off; it is true for a surviving
	// spouse once the other has died.
	Single bool
	// OtherIncome is the income counted by the GIS test, excluding OAS and
	// GIS itself.
	OtherIncome float64
}

// GISAnnual returns the non-taxable GIS payable for the year. GIS is usually
// nil for this household, but it is evaluated in every projection year
// because it can reappear for a low-income survivor. The single rate is
// reduced to nil at the published income cut-off; the spouse-of-pensioner
// rate runs down at $0.50 per $1 of other income. The employment-income
// earnings exemption is not modelled because the engine has no employment
// income after retirement.
func GISAnnual(in GISInput) (float64, error) {
	row, rowYear, err := constants.GIS.For(in.Year)
	if err != nil {
		return 0, err
	}
	at := func(v float64) float64 {
		return constants.ForwardIndex(v, constants.BasisCPI, rowYear, in.Year, in.Forward)
	}
	monthly := row.SpouseOfPensionerMonthly
	if in.Single {
		monthly = row.SingleMaxMonthly
		if in.OtherIncome >= at(row.SingleCutoff) {
			return 0, nil
		}
	}
	reduction := gisReductionRate * math.Max(0, in.OtherIncome)
	return math.Max(0, 12*at(monthly)-reduction), nil
}
