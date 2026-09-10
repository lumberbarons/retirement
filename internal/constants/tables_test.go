package constants

import (
	"math"
	"testing"
)

const testEps = 0.005

var testFwd = Forward{CPI: 0.021, Wage: 0.031}

func mustScalarFor(t *testing.T, s Scalar, year int, f Forward) float64 {
	t.Helper()
	v, err := s.For(year, f)
	if err != nil {
		t.Fatalf("%s.For(%d): %v", s.Desc, year, err)
	}
	return v
}

func TestScalar_ExactYear(t *testing.T) {
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"tfsa 2026", mustScalarFor(t, TFSAAnnualLimit, 2026, testFwd), 7000},
		{"rrsp dollar limit 2026", mustScalarFor(t, RRSPDollarLimit, 2026, testFwd), 33810},
		{"rrsp dollar limit 2027", mustScalarFor(t, RRSPDollarLimit, 2027, testFwd), 35390},
		{"ympe 2026", mustScalarFor(t, YMPE, 2026, testFwd), 74600},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestForwardIndex_CPI(t *testing.T) {
	o, err := OAS.For(2026)
	if err != nil {
		t.Fatalf("OAS.For(2026): %v", err)
	}
	got := ForwardIndex(o.ClawbackThreshold, BasisCPI, 2026, 2028, testFwd)
	want := 95323 * math.Pow(1.021, 2)
	if math.Abs(got-want) > testEps {
		t.Fatalf("ForwardIndex(OAS clawback threshold, 2028) = %v, want %v", got, want)
	}
}

func TestScalar_ForwardIndexWage(t *testing.T) {
	got := mustScalarFor(t, RRSPDollarLimit, 2029, testFwd)
	want := 35390 * math.Pow(1.031, 2)
	if math.Abs(got-want) > testEps {
		t.Fatalf("RRSPDollarLimit.For(2029) = %v, want %v", got, want)
	}
}

func TestScalar_FixedBasisUnchanged(t *testing.T) {
	got := mustScalarFor(t, PrescribedRate, 2035, testFwd)
	if got != 0.03 {
		t.Fatalf("PrescribedRate.For(2035) = %v, want 0.03", got)
	}
}

func TestScalar_TFSARoundsToNearest500(t *testing.T) {
	got := mustScalarFor(t, TFSAAnnualLimit, 2030, testFwd)
	if got != 7500 {
		t.Fatalf("TFSAAnnualLimit.For(2030) = %v, want 7500", got)
	}
}

func TestScalar_YearBeforeOldestRowErrors(t *testing.T) {
	if _, err := TFSAAnnualLimit.For(2007, testFwd); err == nil {
		t.Fatal("TFSAAnnualLimit.For(2007) should error, got nil")
	}
}

func TestTFSA_CumulativeRoomThrough2026(t *testing.T) {
	sum := 0.0
	for y := 2009; y <= 2026; y++ {
		sum += mustScalarFor(t, TFSAAnnualLimit, y, testFwd)
	}
	if sum != 109000 {
		t.Fatalf("cumulative TFSA room 2009-2026 = %v, want 109000", sum)
	}
}

func TestRRIFMinimumFactor(t *testing.T) {
	cases := []struct {
		age  int
		want float64
	}{
		{65, 0.04},
		{70, 0.05},
		{71, 0.0528},
		{72, 0.0540},
		{85, 0.0851},
		{95, 0.2000},
		{100, 0.2000},
	}
	for _, tc := range cases {
		got := RRIFMinimumFactor(tc.age)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("RRIFMinimumFactor(%d) = %v, want %v", tc.age, got, tc.want)
		}
	}
}

func TestTaxYear2026(t *testing.T) {
	ty, err := Tax.For(2026)
	if err != nil {
		t.Fatalf("Tax.For(2026): %v", err)
	}
	if len(ty.FedBrackets) != 5 {
		t.Fatalf("federal brackets: got %d, want 5", len(ty.FedBrackets))
	}
	if ty.FedBrackets[0].Upper != 58523 || ty.FedBrackets[0].Rate != 0.14 {
		t.Fatalf("federal first bracket = %+v, want upper 58523 rate 0.14", ty.FedBrackets[0])
	}
	if !math.IsInf(ty.FedBrackets[4].Upper, 1) || ty.FedBrackets[4].Rate != 0.33 {
		t.Fatalf("federal top bracket = %+v, want unbounded at 0.33", ty.FedBrackets[4])
	}
	if len(ty.ONBrackets) != 5 {
		t.Fatalf("ontario brackets: got %d, want 5", len(ty.ONBrackets))
	}
	if ty.ONBrackets[0].Upper != 53891 || ty.ONBrackets[0].Rate != 0.0505 {
		t.Fatalf("ontario first bracket = %+v, want upper 53891 rate 0.0505", ty.ONBrackets[0])
	}
	if ty.ONBrackets[0].Basis != BasisCPI {
		t.Fatalf("ontario first bracket basis = %v, want cpi", ty.ONBrackets[0].Basis)
	}
	if ty.ONBrackets[2].Upper != 150000 || ty.ONBrackets[2].Basis != BasisFixed {
		t.Fatalf("ontario 150000 bracket = %+v, want frozen upper bound", ty.ONBrackets[2])
	}
	if ty.ONSurtaxT1 != 5818 || ty.ONSurtaxT2 != 7446 {
		t.Fatalf("ontario surtax thresholds = %v/%v, want 5818/7446", ty.ONSurtaxT1, ty.ONSurtaxT2)
	}
	if ty.ONHealthPremium[1].From != 20000 || ty.ONHealthPremium[1].To != 36000 {
		t.Fatalf("OHP first band = %+v, want 20000-36000", ty.ONHealthPremium[1])
	}
	if ty.BPAFed != 16452 || ty.BPAON != 12989 {
		t.Fatalf("BPA = %v/%v, want 16452/12989", ty.BPAFed, ty.BPAON)
	}
	if ty.AgeAmountFed != 9208 || ty.AgeAmountON != 6342 {
		t.Fatalf("age amounts = %v/%v, want 9208/6342", ty.AgeAmountFed, ty.AgeAmountON)
	}
	if ty.PensionAmountFed != 2000 || ty.PensionAmountON != 1796 {
		t.Fatalf("pension amounts = %v/%v, want 2000/1796", ty.PensionAmountFed, ty.PensionAmountON)
	}
}

func TestOAS2026(t *testing.T) {
	o, err := OAS.For(2026)
	if err != nil {
		t.Fatalf("OAS.For(2026): %v", err)
	}
	if o.Monthly65to74 != 742.31 || o.Monthly75Plus != 816.54 {
		t.Fatalf("OAS monthly = %v/%v, want 742.31/816.54", o.Monthly65to74, o.Monthly75Plus)
	}
	if o.ClawbackThreshold != 95323 {
		t.Fatalf("OAS clawback threshold = %v, want 95323", o.ClawbackThreshold)
	}
	if o.ClawbackCeiling65to74 != 154708 || o.ClawbackCeiling75Plus != 160647 {
		t.Fatalf("OAS full-clawback ceilings = %v/%v, want 154708/160647 (ESDC)", o.ClawbackCeiling65to74, o.ClawbackCeiling75Plus)
	}
	rate := mustScalarFor(t, OASRecoveryRate, 2026, testFwd)
	if rate != 0.15 {
		t.Fatalf("OAS recovery rate = %v, want 0.15", rate)
	}
}

func TestCPP2026(t *testing.T) {
	c, err := CPP.For(2026)
	if err != nil {
		t.Fatalf("CPP.For(2026): %v", err)
	}
	if c.MaxMonthlyAt65 != 1507.65 {
		t.Fatalf("CPP max at 65 = %v, want 1507.65", c.MaxMonthlyAt65)
	}
	if c.SurvivorUnder65 != 803.54 || c.Survivor65Plus != 904.59 || c.SurvivorCap != 1531.56 {
		t.Fatalf("CPP survivor = %v/%v cap %v, want 803.54/904.59 cap 1531.56", c.SurvivorUnder65, c.Survivor65Plus, c.SurvivorCap)
	}
	if got := mustScalarFor(t, YMPE, 2026, testFwd); got != 74600 {
		t.Fatalf("YMPE = %v, want 74600", got)
	}
	if got := mustScalarFor(t, YAMPE, 2026, testFwd); got != 85000 {
		t.Fatalf("YAMPE = %v, want 85000", got)
	}
}

func TestFPCanada2026(t *testing.T) {
	f, err := FPCanada.For(2026)
	if err != nil {
		t.Fatalf("FPCanada.For(2026): %v", err)
	}
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"inflation", f.Inflation, 0.021},
		{"salary growth", f.SalaryGrowth, 0.031},
		{"short term", f.ShortTerm, 0.024},
		{"fixed income", f.FixedIncome, 0.032},
		{"canadian equity", f.CanadianEquity, 0.063},
		{"us equity", f.USEquity, 0.064},
		{"intl developed equity", f.IntlDevelopedEquity, 0.066},
		{"emerging equity", f.EmergingEquity, 0.075},
		{"borrowing rate", f.BorrowingRate, 0.044},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestDefaultForward(t *testing.T) {
	f, err := DefaultForward(2030)
	if err != nil {
		t.Fatalf("DefaultForward(2030): %v", err)
	}
	if f.CPI != 0.021 || f.Wage != 0.031 {
		t.Fatalf("DefaultForward = %+v, want CPI 0.021 wage 0.031", f)
	}
}
