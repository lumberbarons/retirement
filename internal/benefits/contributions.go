package benefits

import (
	"math"

	"github.com/lumberbarons/retirement/internal/constants"
)

// CPPEmployeeContributions returns the employee's CPP contributions on the
// year's pensionable employment income, split by tax treatment: base is the
// portion creditable at the lowest federal and Ontario rates, enhanced is the
// remainder, deductible from net income. The contribution is the employee
// rate on earnings between the basic exemption and the YMPE, plus the CPP2
// rate on earnings between the YMPE and the YAMPE (governing spec §2.1).
//
// yearFraction prorates the basic exemption over the part of the year worked
// — the mid-year retirement approximation passes 0.5 — matching the monthly
// proration the Act applies. Both portions come back rounded to the cent.
func CPPEmployeeContributions(employmentIncome, yearFraction float64, year int, f constants.Forward) (base, enhanced float64, err error) {
	if employmentIncome <= 0 || yearFraction <= 0 {
		return 0, 0, nil
	}
	ympe, err := constants.YMPE.For(year, f)
	if err != nil {
		return 0, 0, err
	}
	yampe, err := constants.YAMPE.For(year, f)
	if err != nil {
		return 0, 0, err
	}
	ybe, err := constants.YBE.For(year, f)
	if err != nil {
		return 0, 0, err
	}
	rate, err := constants.CPPContributionRate.For(year, f)
	if err != nil {
		return 0, 0, err
	}
	baseRate, err := constants.CPPBaseContributionRate.For(year, f)
	if err != nil {
		return 0, 0, err
	}
	cpp2Rate, err := constants.CPP2ContributionRate.For(year, f)
	if err != nil {
		return 0, 0, err
	}
	contributory := math.Max(0, math.Min(employmentIncome, ympe)-ybe*yearFraction)
	secondTier := math.Max(0, math.Min(employmentIncome, yampe)-ympe)
	total := contributory*rate + secondTier*cpp2Rate
	base = roundCents(contributory * baseRate)
	enhanced = roundCents(total - contributory*baseRate)
	return base, enhanced, nil
}

func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}
