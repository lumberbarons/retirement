// Package benefits computes the government benefits the household receives:
// CPP retirement and survivor pensions, OAS with its recovery tax, and GIS.
// Amounts are nominal dollars for the projection year, built from the dated
// tables in internal/constants and forward-indexed by the basis recorded there
// (ADR-0002, ADR-0003). CPP and OAS receipts are taxable income; GIS is
// non-taxable.
package benefits

import (
	"fmt"
	"math"

	"github.com/lumberbarons/retirement/internal/constants"
)

// CPPFactor returns the actuarial adjustment applied to the age-65 CPP
// retirement pension for a start age from 60 to 70: 0.640 at 60, 1.000 at 65,
// and 1.420 at 70. The monthly adjustment rates are fixed in law and come
// from the dated tables.
func CPPFactor(startAge, year int, f constants.Forward) (float64, error) {
	if startAge < 60 || startAge > 70 {
		return 0, fmt.Errorf("benefits: CPP start age must be between 60 and 70, got %d", startAge)
	}
	if startAge == 65 {
		return 1, nil
	}
	if startAge < 65 {
		rate, err := constants.CPPEarlyAdjustmentMonthly.For(year, f)
		if err != nil {
			return 0, err
		}
		return 1 - rate*12*float64(65-startAge), nil
	}
	rate, err := constants.CPPLateAdjustmentMonthly.For(year, f)
	if err != nil {
		return 0, err
	}
	return 1 + rate*12*float64(startAge-65), nil
}

// CPPAnnual returns the CPP retirement pension payable in year, for a spouse
// whose stated monthly entitlement at 65 is monthlyAt65 (in base-year
// dollars). No pension is payable before the calendar year of the chosen
// start age; after that the entitlement is scaled by the start-age factor and
// indexed in pay to CPI.
func CPPAnnual(monthlyAt65 float64, startAge, birthYear, year, baseYear int, f constants.Forward) (float64, error) {
	factor, err := CPPFactor(startAge, year, f)
	if err != nil {
		return 0, err
	}
	if year < birthYear+startAge {
		return 0, nil
	}
	return 12 * indexCPI(monthlyAt65, baseYear, year, f) * factor, nil
}

// CPPSurvivorAnnual returns the CPP survivor benefit payable in year to a
// surviving spouse, expressed as the top-up the combined survivor/retirement
// cap allows on top of the survivor's own age-65 pension. The cap is applied
// to the age-65 baseline, so deferring one's own CPP to 70 can lift actual
// receipts above it.
func CPPSurvivorAnnual(deceasedMonthlyAt65, survivorMonthlyAt65 float64, survivorBirthYear, year, baseYear int, f constants.Forward) (float64, error) {
	row, rowYear, err := constants.CPP.For(year)
	if err != nil {
		return 0, err
	}
	table := func(v float64) float64 {
		return constants.ForwardIndex(v, constants.BasisCPI, rowYear, year, f)
	}
	under65Share, err := constants.CPPSurvivorUnder65Share.For(year, f)
	if err != nil {
		return 0, err
	}
	plus65Share, err := constants.CPPSurvivor65PlusShare.For(year, f)
	if err != nil {
		return 0, err
	}
	own := indexCPI(survivorMonthlyAt65, baseYear, year, f)
	deceased := indexCPI(deceasedMonthlyAt65, baseYear, year, f)

	computed := 0.0
	if year-survivorBirthYear >= 65 {
		computed = math.Min(plus65Share*deceased, table(row.Survivor65Plus))
	} else {
		// The flat component reconciles the published under-65 maximum with
		// its flat-plus-share structure: max = flat + share x max pension.
		flat := table(row.SurvivorUnder65) - under65Share*table(row.MaxMonthlyAt65)
		computed = math.Min(flat+under65Share*deceased, table(row.SurvivorUnder65))
	}

	topUp := math.Max(0, math.Min(table(row.SurvivorCap), own+computed)-own)
	return 12 * topUp, nil
}

// ShareCPP applies a user-set CPP sharing election to a pair of retirement
// pensions. fraction is the portion of the combined pensions that is pooled
// and re-split equally; the combined total is unchanged, so the election only
// shifts taxable income from the higher earner to the lower. Zero (the
// default) leaves both pensions untouched.
func ShareCPP(first, second, fraction float64) (float64, float64, error) {
	if fraction < 0 || fraction > 1 {
		return 0, 0, fmt.Errorf("benefits: CPP sharing fraction must be in [0, 1], got %v", fraction)
	}
	shared := fraction * (first + second) / 2
	return (1-fraction)*first + shared, (1-fraction)*second + shared, nil
}

// indexCPI advances a base-year dollar amount to the projection year at CPI,
// the basis every CPP and OAS amount carries.
func indexCPI(v float64, baseYear, year int, f constants.Forward) float64 {
	return constants.ForwardIndex(v, constants.BasisCPI, baseYear, year, f)
}
