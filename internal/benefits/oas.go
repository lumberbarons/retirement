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
// calendar year of the start age. The 75+ rate, which already includes the
// permanent 10% increase, applies from the year the spouse turns 75, and the
// pension is indexed to CPI. Age is taken at December 31.
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
	monthly := row.Monthly65to74
	if year-birthYear >= 75 {
		monthly = row.Monthly75Plus
	}
	monthly = constants.ForwardIndex(monthly, constants.BasisCPI, rowYear, year, f)
	return 12 * monthly * factor, nil
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
