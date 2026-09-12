package tax

import (
	"math"

	"github.com/lumberbarons/retirement/internal/constants"
)

// Income holds one spouse's cash income components for the year, before
// gross-ups, capital-gains inclusion, and pension splitting.
type Income struct {
	Employment           float64
	RRSPWithdrawals      float64
	RRIFWithdrawals      float64
	DBPPension           float64
	CPP                  float64
	OAS                  float64
	Interest             float64
	EligibleDividends    float64
	NonEligibleDividends float64
	CapitalGains         float64

	// CPPContributions is the employee's CPP/CPP2 contribution for the year,
	// a cash outflow that does not enter income. CPPBaseContributions is the
	// portion creditable at the lowest federal and Ontario rates; the rest is
	// the enhanced contribution deducted from net income (line 22215).
	CPPContributions     float64
	CPPBaseContributions float64
}

// EligiblePension returns the income eligible for the pension income amount
// and a T1032 split: DB/RPP pension at any age, RRIF income at 65+. RRSP
// withdrawals, CPP, and OAS never qualify.
func (in Income) EligiblePension(age int) float64 {
	eligible := in.DBPPension
	if age >= pensionSplitAge {
		eligible += in.RRIFWithdrawals
	}
	return eligible
}

// grossedUp returns total taxable income and the federal and Ontario dividend
// tax credits. Eligible and non-eligible dividends carry their own gross-up
// and credit rates; capital gains include at the year's inclusion rate. The
// enhanced portion of the year's CPP contribution is deducted from income;
// the base portion is claimed as a credit by the caller.
func (in Income) grossedUp(rates constants.IncomeYear) (taxable, fedDTC, onDTC float64) {
	grossedEligible := in.EligibleDividends * (1 + rates.GrossUpEligible)
	grossedNonEligible := in.NonEligibleDividends * (1 + rates.GrossUpNonEligible)
	enhancedCPP := math.Max(0, in.CPPContributions-in.CPPBaseContributions)
	taxable = in.Employment + in.RRSPWithdrawals + in.RRIFWithdrawals +
		in.DBPPension + in.CPP + in.OAS + in.Interest +
		grossedEligible + grossedNonEligible +
		in.CapitalGains*rates.CapGainsInclusion - enhancedCPP
	fedDTC = grossedEligible*rates.DTCFedEligible + grossedNonEligible*rates.DTCFedNonEligible
	onDTC = grossedEligible*rates.DTCOnEligible + grossedNonEligible*rates.DTCOnNonEligible
	return taxable, fedDTC, onDTC
}
