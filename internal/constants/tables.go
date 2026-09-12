package constants

import (
	"errors"
	"math"
	"strconv"
)

type Basis string

const (
	BasisCPI          Basis = "cpi"
	BasisAverageWage  Basis = "average_wage"
	BasisFixed        Basis = "fixed"
	BasisPlanSpecific Basis = "plan_specific"
	BasisUserSet      Basis = "user_set"
)

type Forward struct {
	CPI  float64
	Wage float64
}

type Scalar struct {
	Desc         string
	Basis        Basis
	Source       string
	LastVerified string
	Rows         map[int]float64
	Round        func(float64) float64
}

type Table[T any] struct {
	Desc         string
	Basis        Basis
	Source       string
	LastVerified string
	Rows         map[int]T
}

func RoundToNearest500(v float64) float64 {
	return math.Round(v/500) * 500
}

var TFSAAnnualLimit = Scalar{
	Desc:         "TFSA annual dollar limit",
	Basis:        BasisCPI,
	Source:       "CRA",
	LastVerified: "2026-09",
	Round:        RoundToNearest500,
	Rows: map[int]float64{
		2009: 5000, 2010: 5000, 2011: 5000, 2012: 5000,
		2013: 5500, 2014: 5500, 2015: 10000, 2016: 5500, 2017: 5500,
		2018: 5500, 2019: 6000, 2020: 6000, 2021: 6000, 2022: 6000,
		2023: 6500, 2024: 7000, 2025: 7000, 2026: 7000,
	},
}

var RRSPDollarLimit = Scalar{
	Desc:         "RRSP dollar limit",
	Basis:        BasisAverageWage,
	Source:       "CRA",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2025: 32490, 2026: 33810, 2027: 35390},
}

var RRSPEarnedRate = Scalar{
	Desc:         "RRSP room accrual rate on earned income",
	Basis:        BasisFixed,
	Source:       "Income Tax Act",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.18},
}

var RRSPCushion = Scalar{
	Desc:         "RRSP lifetime over-contribution cushion",
	Basis:        BasisFixed,
	Source:       "Income Tax Act",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 2000},
}

var PAMultiplier = Scalar{
	Desc:         "pension adjustment multiplier on annual accrued benefit",
	Basis:        BasisFixed,
	Source:       "Income Tax Act",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 9},
}

var PAOffset = Scalar{
	Desc:         "pension adjustment offset",
	Basis:        BasisFixed,
	Source:       "Income Tax Act",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 600},
}

var RRIFMinFactor = map[int]float64{
	71: 0.0528, 72: 0.0540, 73: 0.0553, 74: 0.0567, 75: 0.0582,
	76: 0.0598, 77: 0.0617, 78: 0.0636, 79: 0.0658, 80: 0.0682,
	81: 0.0708, 82: 0.0738, 83: 0.0771, 84: 0.0808, 85: 0.0851,
	86: 0.0899, 87: 0.0955, 88: 0.1021, 89: 0.1099, 90: 0.1192,
	91: 0.1306, 92: 0.1449, 93: 0.1634, 94: 0.1879, 95: 0.2000,
}

type Bracket struct {
	Lower float64
	Upper float64
	Rate  float64
	Basis Basis
}

type OHPBand struct {
	From float64
	To   float64
	Base float64
	Rate float64
	Cap  float64
}

type TaxYear struct {
	FedBrackets            []Bracket
	ONBrackets             []Bracket
	ONSurtaxT1             float64
	ONSurtaxT2             float64
	ONHealthPremium        []OHPBand
	BPAFed                 float64
	BPAFedPhaseOutFrom     float64
	BPAFedPhaseOutTo       float64
	BPAFedAtPhaseOut       float64
	BPAON                  float64
	AgeAmountFed           float64
	AgePhaseOutFed         float64
	AgeNilFed              float64
	AgeAmountON            float64
	AgePhaseOutON          float64
	AgeNilON               float64
	PensionAmountFed       float64
	PensionAmountON        float64
	SpousalAmountFed       float64
	SpousalAmountON        float64
	SpousalIgnoreON        float64
	CanadaEmploymentAmount float64
}

var Tax = Table[TaxYear]{
	Desc:         "federal and Ontario tax year constants",
	Basis:        BasisCPI,
	Source:       "CRA T4032-ON, current-year tax rates and income brackets (2026)",
	LastVerified: "2026-09",
	Rows: map[int]TaxYear{2026: {
		FedBrackets: []Bracket{
			{Lower: 0, Upper: 58523, Rate: 0.14, Basis: BasisCPI},
			{Lower: 58523, Upper: 117045, Rate: 0.205, Basis: BasisCPI},
			{Lower: 117045, Upper: 181440, Rate: 0.26, Basis: BasisCPI},
			{Lower: 181440, Upper: 258482, Rate: 0.29, Basis: BasisCPI},
			{Lower: 258482, Upper: math.Inf(1), Rate: 0.33, Basis: BasisCPI},
		},
		ONBrackets: []Bracket{
			{Lower: 0, Upper: 53891, Rate: 0.0505, Basis: BasisCPI},
			{Lower: 53891, Upper: 107785, Rate: 0.0915, Basis: BasisCPI},
			{Lower: 107785, Upper: 150000, Rate: 0.1116, Basis: BasisFixed},
			{Lower: 150000, Upper: 220000, Rate: 0.1216, Basis: BasisFixed},
			{Lower: 220000, Upper: math.Inf(1), Rate: 0.1316, Basis: BasisFixed},
		},
		ONSurtaxT1: 5818,
		ONSurtaxT2: 7446,
		ONHealthPremium: []OHPBand{
			{From: 0, To: 20000, Base: 0, Rate: 0, Cap: 0},
			{From: 20000, To: 36000, Base: 0, Rate: 0.06, Cap: 300},
			{From: 36000, To: 48000, Base: 300, Rate: 0.06, Cap: 450},
			{From: 48000, To: 72000, Base: 450, Rate: 0.25, Cap: 600},
			{From: 72000, To: 200000, Base: 600, Rate: 0.25, Cap: 750},
			{From: 200000, To: math.Inf(1), Base: 750, Rate: 0.25, Cap: 900},
		},
		BPAFed:                 16452,
		BPAFedPhaseOutFrom:     181440,
		BPAFedPhaseOutTo:       258482,
		BPAFedAtPhaseOut:       14829,
		BPAON:                  12989,
		AgeAmountFed:           9208,
		AgePhaseOutFed:         46432,
		AgeNilFed:              107819,
		AgeAmountON:            6342,
		AgePhaseOutON:          47210,
		AgeNilON:               89490,
		PensionAmountFed:       2000,
		PensionAmountON:        1796,
		SpousalAmountFed:       16452,
		SpousalAmountON:        11029,
		SpousalIgnoreON:        1103,
		CanadaEmploymentAmount: 1501,
	}},
}

type IncomeYear struct {
	GrossUpEligible    float64
	DTCFedEligible     float64
	DTCOnEligible      float64
	GrossUpNonEligible float64
	DTCFedNonEligible  float64
	DTCOnNonEligible   float64
	CapGainsInclusion  float64
}

var Income = Table[IncomeYear]{
	Desc:         "dividend gross-up, DTC, and capital gains inclusion rates",
	Basis:        BasisFixed,
	Source:       "CRA",
	LastVerified: "2026-09",
	Rows: map[int]IncomeYear{2026: {
		GrossUpEligible:    0.38,
		DTCFedEligible:     0.150198,
		DTCOnEligible:      0.10,
		GrossUpNonEligible: 0.15,
		DTCFedNonEligible:  0.090301,
		DTCOnNonEligible:   0.029863,
		CapGainsInclusion:  0.50,
	}},
}

type CPPYear struct {
	MaxMonthlyAt65  float64
	AvgMonthlyAt65  float64
	PRBMaxMonthly   float64
	SurvivorUnder65 float64
	Survivor65Plus  float64
	SurvivorCap     float64
}

var CPP = Table[CPPYear]{
	Desc:         "CPP benefit amounts",
	Basis:        BasisCPI,
	Source:       "ESDC 2026 rate card",
	LastVerified: "2026-09",
	Rows: map[int]CPPYear{2026: {
		MaxMonthlyAt65:  1507.65,
		AvgMonthlyAt65:  925.35,
		PRBMaxMonthly:   54.69,
		SurvivorUnder65: 803.54,
		Survivor65Plus:  904.59,
		SurvivorCap:     1531.56,
	}},
}

var YMPE = Scalar{
	Desc:         "yearly maximum pensionable earnings",
	Basis:        BasisAverageWage,
	Source:       "ESDC 2026 rate card",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 74600},
}

var YAMPE = Scalar{
	Desc:         "second earnings ceiling for CPP2",
	Basis:        BasisAverageWage,
	Source:       "ESDC 2026 rate card",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 85000},
}

var YBE = Scalar{
	Desc:         "yearly basic exemption",
	Basis:        BasisFixed,
	Source:       "ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 3500},
}

var CPPDeathBenefit = Scalar{
	Desc:         "CPP death benefit lump sum",
	Basis:        BasisFixed,
	Source:       "ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 2500},
}

var CPPEarlyAdjustmentMonthly = Scalar{
	Desc:         "CPP actuarial adjustment per month before 65",
	Basis:        BasisFixed,
	Source:       "ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.006},
}

var CPPLateAdjustmentMonthly = Scalar{
	Desc:         "CPP actuarial adjustment per month after 65",
	Basis:        BasisFixed,
	Source:       "ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.007},
}

var CPPSurvivorUnder65Share = Scalar{
	Desc:         "CPP survivor share of the deceased's retirement pension, survivor under 65",
	Basis:        BasisFixed,
	Source:       "ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.375},
}

var CPPSurvivor65PlusShare = Scalar{
	Desc:         "CPP survivor share of the deceased's retirement pension, survivor 65 or older",
	Basis:        BasisFixed,
	Source:       "ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.60},
}

type OASYear struct {
	Monthly65to74         float64
	Monthly75Plus         float64
	ClawbackThreshold     float64
	ClawbackCeiling65to74 float64
	ClawbackCeiling75Plus float64
}

var OAS = Table[OASYear]{
	Desc:         "OAS amounts and clawback thresholds",
	Basis:        BasisCPI,
	Source:       "ESDC quarterly rate card Jan-Mar 2026 (annual model applies the Jan rate for the full year)",
	LastVerified: "2026-09",
	Rows: map[int]OASYear{2026: {
		Monthly65to74:         742.31,
		Monthly75Plus:         816.54,
		ClawbackThreshold:     95323,
		ClawbackCeiling65to74: 154708,
		ClawbackCeiling75Plus: 160647,
	}},
}

var OASRecoveryRate = Scalar{
	Desc:         "OAS recovery tax rate",
	Basis:        BasisFixed,
	Source:       "CRA / ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.15},
}

var OASDeferralMonthly = Scalar{
	Desc:         "OAS deferral increase per month past 65",
	Basis:        BasisFixed,
	Source:       "ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.006},
}

var GISReductionRate = Scalar{
	Desc:         "GIS reduction per dollar of other income",
	Basis:        BasisFixed,
	Source:       "ESDC",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.50},
}

type GISYear struct {
	SingleMaxMonthly         float64
	SingleCutoff             float64
	SpouseOfPensionerMonthly float64
}

var GIS = Table[GISYear]{
	Desc:         "GIS amounts and income cut-off",
	Basis:        BasisCPI,
	Source:       "ESDC 2026 rate card Jan-Mar",
	LastVerified: "2026-09",
	Rows: map[int]GISYear{2026: {
		SingleMaxMonthly:         1108.74,
		SingleCutoff:             22488,
		SpouseOfPensionerMonthly: 667.41,
	}},
}

var PrescribedRate = Scalar{
	Desc:         "CRA prescribed interest rate for new spousal loans",
	Basis:        BasisFixed,
	Source:       "CRA prescribed rates, Q4 2026 (set quarterly; locked for the life of a loan at inception)",
	LastVerified: "2026-09",
	Rows:         map[int]float64{2026: 0.03},
}

type FPCanadaYear struct {
	Inflation           float64
	SalaryGrowth        float64
	ShortTerm           float64
	FixedIncome         float64
	CanadianEquity      float64
	USEquity            float64
	IntlDevelopedEquity float64
	EmergingEquity      float64
	BorrowingRate       float64
}

var FPCanada = Table[FPCanadaYear]{
	Desc:         "FP Canada 2026 Projection Assumption Guidelines, long-term 10yr+ before fees (April 2026)",
	Basis:        BasisUserSet,
	Source:       "FP Canada / Institute of Financial Planning",
	LastVerified: "2026-09",
	Rows: map[int]FPCanadaYear{2026: {
		Inflation:           0.021,
		SalaryGrowth:        0.031,
		ShortTerm:           0.024,
		FixedIncome:         0.032,
		CanadianEquity:      0.063,
		USEquity:            0.064,
		IntlDevelopedEquity: 0.066,
		EmergingEquity:      0.075,
		BorrowingRate:       0.044,
	}},
}

func (s Scalar) For(year int, f Forward) (float64, error) {
	y, ok := latestKey(s.Rows, year)
	if !ok {
		return 0, errors.New(s.Desc + ": no value at or before " + strconv.Itoa(year))
	}
	v := ForwardIndex(s.Rows[y], s.Basis, y, year, f)
	if s.Round != nil {
		v = s.Round(v)
	}
	return v, nil
}

func ForwardIndex(v float64, b Basis, from, to int, f Forward) float64 {
	if to <= from {
		return v
	}
	switch b {
	case BasisCPI:
		return v * math.Pow(1+f.CPI, float64(to-from))
	case BasisAverageWage:
		return v * math.Pow(1+f.Wage, float64(to-from))
	default:
		return v
	}
}

// For returns the newest row at or before year, along with that row's year.
// Composite rows mix index bases (e.g. Tax has CPI federal brackets beside
// frozen Ontario thresholds), so the row is returned unindexed: callers must
// forward-index each field from the returned year to the requested year
// according to that field's basis (see ForwardIndex). Rows whose basis is
// fixed or user-set are valid unchanged for any later year.
func (t Table[T]) For(year int) (T, int, error) {
	var zero T
	y, ok := latestKey(t.Rows, year)
	if !ok {
		return zero, 0, errors.New(t.Desc + ": no value at or before " + strconv.Itoa(year))
	}
	return t.Rows[y], y, nil
}

func DefaultForward(year int) (Forward, error) {
	f, _, err := FPCanada.For(year)
	if err != nil {
		return Forward{}, err
	}
	return Forward{CPI: f.Inflation, Wage: f.SalaryGrowth}, nil
}

func RRIFMinimumFactor(age int) float64 {
	if age <= 70 {
		return 1.0 / float64(90-age)
	}
	if f, ok := RRIFMinFactor[age]; ok {
		return f
	}
	return 0.2000
}

func latestKey[V any](m map[int]V, year int) (int, bool) {
	best, found := 0, false
	for y := range m {
		if y <= year && (!found || y > best) {
			best, found = y, true
		}
	}
	return best, found
}
