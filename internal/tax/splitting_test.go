package tax

import "testing"

func TestApplyPensionSplit_DefaultZeroChangesNothing(t *testing.T) {
	incomes := []Income{
		{RRIFWithdrawals: 60000, DBPPension: 10000},
		{RRIFWithdrawals: 20000},
	}
	ages := []int{66, 68}
	got := ApplyPensionSplit(incomes, ages, []float64{0, 0})
	if got[0] != incomes[0] || got[1] != incomes[1] {
		t.Fatalf("zero split changed incomes: %+v", got)
	}
}

func TestApplyPensionSplit_MovesRRIFAt65(t *testing.T) {
	incomes := []Income{{RRIFWithdrawals: 100000}, {}}
	got := ApplyPensionSplit(incomes, []int{70, 70}, []float64{0.5, 0})
	almostEqual(t, got[0].RRIFWithdrawals, 50000)
	almostEqual(t, got[1].RRIFWithdrawals, 50000)
}

func TestApplyPensionSplit_RRIFNotEligibleBefore65(t *testing.T) {
	incomes := []Income{{RRIFWithdrawals: 100000}, {}}
	got := ApplyPensionSplit(incomes, []int{60, 70}, []float64{0.5, 0})
	almostEqual(t, got[0].RRIFWithdrawals, 100000)
	almostEqual(t, got[1].RRIFWithdrawals, 0)
}

func TestApplyPensionSplit_DBQualifiesAtAnyAge(t *testing.T) {
	incomes := []Income{{DBPPension: 40000}, {}}
	got := ApplyPensionSplit(incomes, []int{55, 55}, []float64{0.5, 0})
	almostEqual(t, got[0].DBPPension, 20000)
	almostEqual(t, got[1].DBPPension, 20000)
}

func TestApplyPensionSplit_NeverMovesRRSPCPPOAS(t *testing.T) {
	incomes := []Income{{RRSPWithdrawals: 10000, CPP: 5000, OAS: 3000}, {}}
	got := ApplyPensionSplit(incomes, []int{70, 70}, []float64{0.5, 0})
	almostEqual(t, got[0].RRSPWithdrawals, 10000)
	almostEqual(t, got[0].CPP, 5000)
	almostEqual(t, got[0].OAS, 3000)
	if got[1].RRSPWithdrawals != 0 || got[1].CPP != 0 || got[1].OAS != 0 {
		t.Fatalf("RRSP/CPP/OAS moved: %+v", got[1])
	}
}

func TestApplyPensionSplit_TwoWayUsesOriginalAmounts(t *testing.T) {
	incomes := []Income{{RRIFWithdrawals: 100000}, {RRIFWithdrawals: 40000}}
	got := ApplyPensionSplit(incomes, []int{70, 70}, []float64{0.5, 0.5})
	almostEqual(t, got[0].RRIFWithdrawals, 70000)
	almostEqual(t, got[1].RRIFWithdrawals, 70000)
}
