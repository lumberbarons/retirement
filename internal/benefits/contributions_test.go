package benefits

import (
	"math"
	"testing"

	"github.com/lumberbarons/retirement/internal/constants"
)

func TestCPPEmployeeContributions(t *testing.T) {
	fwd := constants.Forward{CPI: 0.021, Wage: 0.031}
	cases := []struct {
		name      string
		income    float64
		fraction  float64
		wantBase  float64
		wantEnhd  float64
		wantTotal float64
	}{
		{
			name: "no employment income", income: 0, fraction: 1,
			wantBase: 0, wantEnhd: 0, wantTotal: 0,
		},
		{
			name: "below the basic exemption", income: 3000, fraction: 1,
			wantBase: 0, wantEnhd: 0, wantTotal: 0,
		},
		{
			name: "between the exemption and the YMPE", income: 50000, fraction: 1,
			wantBase: 46500 * 0.0495, wantEnhd: 46500 * 0.01, wantTotal: 46500 * 0.0595,
		},
		{
			name: "above the YAMPE", income: 120000, fraction: 1,
			wantBase: 71100 * 0.0495, wantEnhd: 71100*0.01 + 10400*0.04, wantTotal: 71100*0.0595 + 10400*0.04,
		},
		{
			name: "half-year retirement prorates the exemption", income: 50000, fraction: 0.5,
			wantBase: 48250 * 0.0495, wantEnhd: 48250 * 0.01, wantTotal: 48250 * 0.0595,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, enhanced, err := CPPEmployeeContributions(tc.income, tc.fraction, 2026, fwd)
			if err != nil {
				t.Fatalf("CPPEmployeeContributions: %v", err)
			}
			if math.Abs(base-roundToCent(tc.wantBase)) > 0.001 {
				t.Fatalf("base = %v, want %v", base, roundToCent(tc.wantBase))
			}
			if math.Abs(enhanced-roundToCent(tc.wantEnhd)) > 0.001 {
				t.Fatalf("enhanced = %v, want %v", enhanced, roundToCent(tc.wantEnhd))
			}
			if math.Abs(base+enhanced-roundToCent(tc.wantTotal)) > 0.005 {
				t.Fatalf("total = %v, want %v", base+enhanced, roundToCent(tc.wantTotal))
			}
		})
	}
}

func TestCPPEmployeeContributions_ProratesWithIncomeAndCaps(t *testing.T) {
	fwd := constants.Forward{}
	atCapBase, _, err := CPPEmployeeContributions(85000, 1, 2026, fwd)
	if err != nil {
		t.Fatalf("CPPEmployeeContributions: %v", err)
	}
	aboveCapBase, _, err := CPPEmployeeContributions(500000, 1, 2026, fwd)
	if err != nil {
		t.Fatalf("CPPEmployeeContributions: %v", err)
	}
	if math.Abs(atCapBase-aboveCapBase) > 0.001 {
		t.Fatalf("base at YAMPE %v differs from an unbounded income %v", atCapBase, aboveCapBase)
	}
}

func roundToCent(v float64) float64 {
	return math.Round(v*100) / 100
}
