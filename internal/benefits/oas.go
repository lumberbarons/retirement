package benefits

import (
	"fmt"
	"math"

	"github.com/lumberbarons/retirement/internal/constants"
)

// OASFactor returns the deferral factor applied to the age-65 OAS pension for
// a start age from 65 to 70: 1.000 at 65, rising 0.6% per month to 1.360 at
// 70. The monthly deferral rate comes from the dated tables.
func OASFactor(startAge, year int, f constants.Forward) (float64, error) {
	if startAge < 65 || startAge > 70 {
		return 0, fmt.Errorf("benefits: OAS start age must be between 65 and 70, got %d", startAge)
	}
	rate, err := constants.OASDeferralMonthly.For(year, f)
	if err != nil {
		return 0, err
	}
	return 1 + rate*12*float64(startAge-65), nil
}

// OASAnnual returns the OAS pension payable in year before the recovery tax,
// for a spouse with the chosen start age. No pension is payable before the
// calendar year of the start age. The year's pension is the sum of the four
// quarterly rates, not twelve times the January rate; after the newest table
// row the four-quarter total grows by one annual CPI factor per year — the
// documented convention — so the quarterly shape is never re-applied within a
// future year and the total is not double-indexed. The 75+ rates, which
// already include the permanent 10% increase, apply from the year the spouse
// turns 75. Age is taken at December 31, and the start year pays the full
// annual amount rather than a partial one.
func OASAnnual(startAge, birthYear, year int, f constants.Forward) (float64, error) {
	factor, err := OASFactor(startAge, year, f)
	if err != nil {
		return 0, err
	}
	if year < birthYear+startAge {
		return 0, nil
	}
	row, rowYear, err := constants.OAS.For(year)
	if err != nil {
		return 0, err
	}
	quarters := row.Quarterly65to74
	if year-birthYear >= 75 {
		quarters = row.Quarterly75Plus
	}
	annual := 0.0
	for _, monthly := range quarters {
		annual += 3 * monthly
	}
	annual = constants.ForwardIndex(annual, constants.BasisCPI, rowYear, year, f)
	return annual * factor, nil
}

// OASRecovery returns the recovery tax on the year's OAS pension: 15% of net
// income over the threshold, capped at the pension itself. Net income includes
// the OAS pension, so the recovery is evaluated once the year's income is
// known. The pension is fully clawed back at the ESDC full-clawback ceilings.
func OASRecovery(oasReceived, netIncome float64, year int, f constants.Forward) (float64, error) {
	row, rowYear, err := constants.OAS.For(year)
	if err != nil {
		return 0, err
	}
	rate, err := constants.OASRecoveryRate.For(year, f)
	if err != nil {
		return 0, err
	}
	threshold := constants.ForwardIndex(row.ClawbackThreshold, constants.BasisCPI, rowYear, year, f)
	recovery := rate * math.Max(0, netIncome-threshold)
	return math.Min(math.Max(0, oasReceived), recovery), nil
}
