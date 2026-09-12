package tax

import (
	"fmt"
	"math"

	"github.com/lumberbarons/retirement/internal/constants"
)

// Household is one tax year's joint filing picture: both spouses' incomes,
// ages, and elected T1032 split fractions.
type Household struct {
	Year    int
	Forward constants.Forward
	Spouses []Spouse
}

// Spouse is one spouse's input to the household tax computation. Age is the
// age at December 31 of the tax year, which the engine uses for credit and
// pension eligibility.
type Spouse struct {
	Name                 string
	Age                  int
	Income               Income
	PensionSplitFraction float64
}

// Result is the household's tax for the year, with one entry per spouse in
// the input order.
type Result struct {
	Year    int
	Total   float64
	Spouses []SpouseResult
}

// SpouseResult breaks one spouse's tax into the jurisdiction detail the
// validation suite and the report need.
type SpouseResult struct {
	Name                 string
	TaxableIncome        float64
	PensionTransferred   float64
	PensionReceived      float64
	CreditTransferredOut float64
	CreditTransferredIn  float64

	FederalTaxBeforeCredits  float64
	FederalDividendTaxCredit float64
	FederalCredits           float64
	FederalTax               float64

	OntarioBasicTax             float64
	OntarioDividendTaxCredit    float64
	OntarioCredits              float64
	OntarioBasicTaxAfterCredits float64
	OntarioSurtax               float64
	OntarioHealthPremium        float64
	OntarioTax                  float64

	TotalTax float64

	fedUnusedTransferable float64
	onUnusedTransferable  float64
}

// Compute returns the household's federal and Ontario tax for the year. The
// Ontario surtax is applied to basic Ontario tax after non-refundable
// credits, and unused age and pension credits of the lower-income spouse
// transfer to the higher-income spouse.
func Compute(h Household) (Result, error) {
	if len(h.Spouses) != 2 {
		return Result{}, fmt.Errorf("tax: exactly 2 spouses are required, got %d", len(h.Spouses))
	}
	for _, sp := range h.Spouses {
		if sp.PensionSplitFraction < 0 || sp.PensionSplitFraction > maxPensionSplitFraction {
			return Result{}, fmt.Errorf("tax: pension split fraction for %s must be in [0, 0.5], got %v", sp.Name, sp.PensionSplitFraction)
		}
	}
	iy, err := indexYear(h.Year, h.Forward)
	if err != nil {
		return Result{}, err
	}
	incomes := make([]Income, len(h.Spouses))
	ages := make([]int, len(h.Spouses))
	fractions := make([]float64, len(h.Spouses))
	for i, sp := range h.Spouses {
		incomes[i] = sp.Income
		ages[i] = sp.Age
		fractions[i] = sp.PensionSplitFraction
	}
	split := ApplyPensionSplit(incomes, ages, fractions)
	results := make([]SpouseResult, len(h.Spouses))
	for i, sp := range h.Spouses {
		spouseNet := 0.0
		for j := range h.Spouses {
			if j != i {
				spouseNet, _, _ = split[j].grossedUp(iy.income)
			}
		}
		results[i] = computeSpouse(sp.Name, ages[i], split[i], spouseNet, iy)
		original := incomes[i].EligiblePension(ages[i])
		after := split[i].EligiblePension(ages[i])
		switch {
		case after > original:
			results[i].PensionReceived = after - original
		default:
			results[i].PensionTransferred = original - after
		}
	}
	transferCredits(results, iy)
	res := Result{Year: h.Year, Spouses: results}
	for _, sr := range results {
		res.Total += sr.TotalTax
	}
	return res, nil
}

// computeSpouse computes one spouse's federal and Ontario tax, given the
// other spouse's post-split taxable income for the spousal amount.
func computeSpouse(name string, age int, in Income, spouseNetIncome float64, iy *indexedYear) SpouseResult {
	taxable, fedDTC, onDTC := in.grossedUp(iy.income)
	r := SpouseResult{
		Name:                     name,
		TaxableIncome:            taxable,
		FederalTaxBeforeCredits:  BracketTax(taxable, iy.fedBrackets),
		FederalDividendTaxCredit: fedDTC,
		OntarioBasicTax:          BracketTax(taxable, iy.onBrackets),
		OntarioDividendTaxCredit: onDTC,
		OntarioHealthPremium:     HealthPremium(taxable, iy.onHealth),
	}

	fedAge := ageAmount(taxable, age, iy.ageFed, iy.agePhaseOutFed, iy.ageNilFed)
	fedPension := math.Min(iy.pensionFed, in.EligiblePension(age))
	fedTransferable := fedAge + fedPension
	fedCreditBase := iy.federalBPA(taxable) + fedTransferable +
		spousalAmount(spouseNetIncome, iy.spousalFed, 0) +
		math.Min(in.Employment, iy.employment) +
		in.CPPBaseContributions
	fedCredits := fedCreditBase * iy.fedBrackets[0].Rate
	fedAvailable := math.Max(0, r.FederalTaxBeforeCredits-fedDTC)
	r.FederalCredits = math.Min(fedCredits, fedAvailable)
	r.FederalTax = fedAvailable - r.FederalCredits
	r.fedUnusedTransferable = math.Min(fedTransferable*iy.fedBrackets[0].Rate, math.Max(0, fedCredits-fedAvailable))

	onAge := ageAmount(taxable, age, iy.ageON, iy.agePhaseOutON, iy.ageNilON)
	onPension := math.Min(iy.pensionON, in.EligiblePension(age))
	onTransferable := onAge + onPension
	onCreditBase := iy.bpaON + onTransferable +
		spousalAmount(spouseNetIncome, iy.spousalON, iy.spousalIgnON) +
		in.CPPBaseContributions
	onCredits := onCreditBase * iy.onBrackets[0].Rate
	onAvailable := math.Max(0, r.OntarioBasicTax-onDTC)
	r.OntarioCredits = math.Min(onCredits, onAvailable)
	r.OntarioBasicTaxAfterCredits = onAvailable - r.OntarioCredits
	r.OntarioSurtax = Surtax(r.OntarioBasicTaxAfterCredits, iy.onSurtaxT1, iy.onSurtaxT2)
	r.OntarioTax = r.OntarioBasicTaxAfterCredits + r.OntarioSurtax + r.OntarioHealthPremium
	r.onUnusedTransferable = math.Min(onTransferable*iy.onBrackets[0].Rate, math.Max(0, onCredits-onAvailable))

	r.TotalTax = r.FederalTax + r.OntarioTax
	return r
}

// transferCredits moves the lower-income spouse's unused age and pension
// credits to the higher-income spouse, per Schedule 2. The Ontario transfer
// reduces basic Ontario tax before the surtax is recomputed, so crossing a
// surtax band is respected.
func transferCredits(results []SpouseResult, iy *indexedYear) {
	hi, lo := 0, 1
	if results[lo].TaxableIncome > results[hi].TaxableIncome {
		hi, lo = lo, hi
	}
	fedIn := math.Min(results[lo].fedUnusedTransferable, results[hi].FederalTax)
	onIn := math.Min(results[lo].onUnusedTransferable, results[hi].OntarioBasicTaxAfterCredits)
	if fedIn <= 0 && onIn <= 0 {
		return
	}
	results[hi].FederalTax -= fedIn
	results[hi].OntarioBasicTaxAfterCredits -= onIn
	results[hi].OntarioSurtax = Surtax(results[hi].OntarioBasicTaxAfterCredits, iy.onSurtaxT1, iy.onSurtaxT2)
	results[hi].OntarioTax = results[hi].OntarioBasicTaxAfterCredits + results[hi].OntarioSurtax + results[hi].OntarioHealthPremium
	results[hi].TotalTax = results[hi].FederalTax + results[hi].OntarioTax
	results[lo].CreditTransferredOut = fedIn + onIn
	results[hi].CreditTransferredIn = fedIn + onIn
}

// Surtax applies Ontario's two-tier surtax to basic Ontario tax after
// non-refundable credits. The ordering matters: computing it before credits
// would move a retiree between surtax bands.
func Surtax(onTaxAfterCredits, tier1, tier2 float64) float64 {
	return 0.20*math.Max(0, onTaxAfterCredits-tier1) +
		0.36*math.Max(0, onTaxAfterCredits-tier2)
}

// HealthPremium applies the Ontario Health Premium schedule to taxable
// income. The bands are fixed in law and never indexed.
func HealthPremium(taxable float64, bands []constants.OHPBand) float64 {
	for _, b := range bands {
		if taxable >= b.From && taxable <= b.To {
			return math.Min(b.Cap, b.Base+b.Rate*(taxable-b.From))
		}
	}
	return 0
}
