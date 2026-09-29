package benefits

import (
	"math"

	"github.com/lumberbarons/retirement/internal/constants"
)

// GISInput is one spouse-year's Guaranteed Income Supplement test.
type GISInput struct {
	Year    int
	Forward constants.Forward
	// Single applies the single schedule and cut-off; it is true for a
	// surviving spouse once the other has died.
	Single bool
	// OASReceived reports whether the spouse receives an OAS pension for the
	// year. GIS is payable only alongside OAS, so no benefit accrues before
	// the spouse's OAS start year.
	OASReceived bool
	// Income is the spouse's income for the test, excluding OAS and GIS.
	Income float64
	// EmploymentIncome is the employment part of Income.
	EmploymentIncome float64
	// PartnerIncome and PartnerEmploymentIncome are the other spouse's
	// corresponding figures. When Single is false the two incomes are
	// combined for the spouse-of-pensioner test, each net of its own
	// earnings exemption.
	PartnerIncome           float64
	PartnerEmploymentIncome float64
}

// GISAnnual returns the non-taxable GIS payable for the year to one spouse.
// GIS requires receipt of an OAS pension, so it is nil before the spouse's
// OAS start year. The income test uses the spouse's own income for a single
// recipient and the couple's combined income for a spouse of a pensioner,
// each income first reduced by the employment earnings exemption. The
// benefit then declines on the published reduction schedule — 50% of income
// in the first band, 75% in the middle band, and 50% above it for a single
// recipient, half those rates on combined income for a spouse of a
// pensioner — and is nil at the published cut-off, so there is no cliff. The
// spouse-of-a-non-pensioner category is not modelled: a couple is tested as
// spouses of pensioners, which is conservative while the younger spouse has
// not reached OAS age (governing spec §2.3; ESDC Jan-Mar 2026 rate card).
func GISAnnual(in GISInput) (float64, error) {
	if !in.OASReceived {
		return 0, nil
	}
	row, rowYear, err := constants.GIS.For(in.Year)
	if err != nil {
		return 0, err
	}
	at := func(v float64) float64 {
		return constants.ForwardIndex(v, constants.BasisCPI, rowYear, in.Year, in.Forward)
	}
	monthly, cutoff, bands := row.SingleMaxMonthly, at(row.SingleCutoff), row.SingleReduction
	if !in.Single {
		monthly, cutoff, bands = row.SpouseOfPensionerMonthly, at(row.SpouseOfPensionerCutoff), row.SpouseReduction
	}

	income := in.Income - employmentExemption(row, in.EmploymentIncome)
	if !in.Single {
		income += in.PartnerIncome - employmentExemption(row, in.PartnerEmploymentIncome)
	}
	income = math.Max(0, income)
	if income >= cutoff {
		return 0, nil
	}

	reduction := 0.0
	for _, b := range bands {
		lower := constants.ForwardIndex(b.Lower, b.Basis, rowYear, in.Year, in.Forward)
		upper := constants.ForwardIndex(b.Upper, b.Basis, rowYear, in.Year, in.Forward)
		if income <= lower {
			break
		}
		reduction += (math.Min(income, upper) - lower) * b.Rate
	}
	return math.Max(0, 12*at(monthly)-reduction), nil
}

// employmentExemption returns the employment income excluded from the GIS
// income test: the first EmploymentExemptFull, plus EmploymentExemptHalfRate
// of the next EmploymentExemptHalf.
func employmentExemption(row constants.GISYear, employment float64) float64 {
	employment = math.Max(0, employment)
	full := math.Min(employment, row.EmploymentExemptFull)
	half := math.Min(math.Max(0, employment-row.EmploymentExemptFull), row.EmploymentExemptHalf)
	return full + row.EmploymentExemptHalfRate*half
}
