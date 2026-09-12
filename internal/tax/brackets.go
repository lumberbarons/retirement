// Package tax computes federal and Ontario personal income tax for the
// two-spouse Ontario household the engine models. Amounts are nominal dollars
// for the projection year, and every dollar figure comes from the dated tables
// in internal/constants, forward-indexed by the basis recorded there
// (ADR-0002, ADR-0003).
package tax

import (
	"math"

	"github.com/lumberbarons/retirement/internal/constants"
)

// pensionSplitAge is the age at which RRIF income becomes eligible for a
// T1032 pension split and for the pension income amount. DB/RPP income is
// eligible at any age; RRSP withdrawals and CPP/OAS never are.
const pensionSplitAge = 65

// maxPensionSplitFraction is the T1032 ceiling: up to half of eligible
// pension income may be transferred to the spouse.
const maxPensionSplitFraction = 0.5

// indexedYear is one projection year's tax constants with every dollar
// threshold already forward-indexed from its dated-table row. Fields whose
// basis is fixed in law keep their table value unchanged.
type indexedYear struct {
	fedBrackets []constants.Bracket
	onBrackets  []constants.Bracket

	onSurtaxT1 float64
	onSurtaxT2 float64
	onHealth   []constants.OHPBand

	bpaFed             float64
	bpaFedPhaseOutFrom float64
	bpaFedPhaseOutTo   float64
	bpaFedAtPhaseOut   float64
	bpaON              float64

	ageFed         float64
	agePhaseOutFed float64
	ageNilFed      float64
	ageON          float64
	agePhaseOutON  float64
	ageNilON       float64

	pensionFed   float64
	pensionON    float64
	spousalFed   float64
	spousalON    float64
	spousalIgnON float64
	employment   float64

	income constants.IncomeYear
}

func indexYear(year int, f constants.Forward) (*indexedYear, error) {
	row, rowYear, err := constants.Tax.For(year)
	if err != nil {
		return nil, err
	}
	inc, _, err := constants.Income.For(year)
	if err != nil {
		return nil, err
	}
	at := func(v float64, basis constants.Basis) float64 {
		return constants.ForwardIndex(v, basis, rowYear, year, f)
	}
	return &indexedYear{
		fedBrackets: IndexBrackets(row.FedBrackets, rowYear, year, f),
		onBrackets:  IndexBrackets(row.ONBrackets, rowYear, year, f),

		onSurtaxT1: at(row.ONSurtaxT1, constants.BasisCPI),
		onSurtaxT2: at(row.ONSurtaxT2, constants.BasisCPI),
		onHealth:   row.ONHealthPremium,

		bpaFed:             at(row.BPAFed, constants.BasisCPI),
		bpaFedPhaseOutFrom: at(row.BPAFedPhaseOutFrom, constants.BasisCPI),
		bpaFedPhaseOutTo:   at(row.BPAFedPhaseOutTo, constants.BasisCPI),
		bpaFedAtPhaseOut:   at(row.BPAFedAtPhaseOut, constants.BasisCPI),
		bpaON:              at(row.BPAON, constants.BasisCPI),

		ageFed:         at(row.AgeAmountFed, constants.BasisCPI),
		agePhaseOutFed: at(row.AgePhaseOutFed, constants.BasisCPI),
		ageNilFed:      at(row.AgeNilFed, constants.BasisCPI),
		ageON:          at(row.AgeAmountON, constants.BasisCPI),
		agePhaseOutON:  at(row.AgePhaseOutON, constants.BasisCPI),
		ageNilON:       at(row.AgeNilON, constants.BasisCPI),

		pensionFed: row.PensionAmountFed,
		pensionON:  at(row.PensionAmountON, constants.BasisCPI),
		spousalFed: at(row.SpousalAmountFed, constants.BasisCPI),
		spousalON:  at(row.SpousalAmountON, constants.BasisCPI),
		// The Ontario spousal offset is approximate and never indexed.
		spousalIgnON: row.SpousalIgnoreON,
		employment:   at(row.CanadaEmploymentAmount, constants.BasisCPI),

		income: inc,
	}, nil
}

// IndexBrackets returns a year's brackets with each boundary forward-indexed
// once and shared with the neighbouring bracket, so the schedule stays
// contiguous. A bracket's basis governs the upper threshold it ends at; its
// lower threshold is inherited from the bracket below. That keeps Ontario's
// $107,785 boundary indexed with the 9.15% bracket while the frozen $150,000
// and $220,000 boundaries above it stay put.
func IndexBrackets(brackets []constants.Bracket, rowYear, year int, f constants.Forward) []constants.Bracket {
	out := make([]constants.Bracket, len(brackets))
	for i, b := range brackets {
		lower := constants.ForwardIndex(b.Lower, b.Basis, rowYear, year, f)
		if i > 0 {
			lower = out[i-1].Upper
		}
		out[i] = constants.Bracket{
			Lower: lower,
			Upper: constants.ForwardIndex(b.Upper, b.Basis, rowYear, year, f),
			Rate:  b.Rate,
			Basis: b.Basis,
		}
	}
	return out
}

// BracketTax applies a progressive bracket schedule to taxable income.
func BracketTax(taxable float64, brackets []constants.Bracket) float64 {
	if taxable <= 0 {
		return 0
	}
	tax := 0.0
	for _, b := range brackets {
		if taxable <= b.Lower {
			break
		}
		tax += (math.Min(taxable, b.Upper) - b.Lower) * b.Rate
	}
	return tax
}

// FederalBrackets returns the federal bracket schedule for the year, indexed
// from its dated-table row.
func FederalBrackets(year int, f constants.Forward) ([]constants.Bracket, error) {
	iy, err := indexYear(year, f)
	if err != nil {
		return nil, err
	}
	return iy.fedBrackets, nil
}

// OntarioBrackets returns the Ontario bracket schedule for the year, indexed
// from its dated-table row (the two lowest thresholds index to CPI; the
// $150,000 and $220,000 thresholds are frozen).
func OntarioBrackets(year int, f constants.Forward) ([]constants.Bracket, error) {
	iy, err := indexYear(year, f)
	if err != nil {
		return nil, err
	}
	return iy.onBrackets, nil
}

// FederalCreditRate is the rate non-refundable credits are valued at: the
// lowest federal bracket rate.
func FederalCreditRate(year int, f constants.Forward) (float64, error) {
	b, err := FederalBrackets(year, f)
	if err != nil {
		return 0, err
	}
	return b[0].Rate, nil
}

// OntarioCreditRate is the rate Ontario non-refundable credits are valued at:
// the lowest Ontario bracket rate.
func OntarioCreditRate(year int, f constants.Forward) (float64, error) {
	b, err := OntarioBrackets(year, f)
	if err != nil {
		return 0, err
	}
	return b[0].Rate, nil
}

// FederalBasicPersonalAmount returns the federal BPA for the year, phased
// down linearly from the full amount to the phase-out amount as taxable
// income crosses the published range.
func FederalBasicPersonalAmount(taxable float64, year int, f constants.Forward) (float64, error) {
	iy, err := indexYear(year, f)
	if err != nil {
		return 0, err
	}
	return iy.federalBPA(taxable), nil
}

func (iy *indexedYear) federalBPA(taxable float64) float64 {
	switch {
	case taxable <= iy.bpaFedPhaseOutFrom:
		return iy.bpaFed
	case taxable >= iy.bpaFedPhaseOutTo:
		return iy.bpaFedAtPhaseOut
	default:
		span := iy.bpaFedPhaseOutTo - iy.bpaFedPhaseOutFrom
		frac := (taxable - iy.bpaFedPhaseOutFrom) / span
		return iy.bpaFed - frac*(iy.bpaFed-iy.bpaFedAtPhaseOut)
	}
}

// ageAmount returns the age credit for a spouse 65 or older, phased out at
// 15% of income over the threshold and nil past the published ceiling.
func ageAmount(netIncome float64, age int, amount, phaseOutFrom, nilAt float64) float64 {
	if age < 65 || netIncome >= nilAt {
		return 0
	}
	return math.Max(0, amount-0.15*math.Max(0, netIncome-phaseOutFrom))
}

// spousalAmount returns the spousal credit amount, reduced dollar for dollar
// by the spouse's income over the ignore band.
func spousalAmount(spouseIncome, amount, ignore float64) float64 {
	return math.Max(0, amount-math.Max(0, spouseIncome-ignore))
}
