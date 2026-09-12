package tax

// ApplyPensionSplit moves each spouse's T1032-elected fraction of their
// eligible pension income to the other spouse. The fraction is limited to
// half; a zero fraction changes nothing. Transfers are computed from the
// pre-split amounts, so a two-way split cannot compound.
func ApplyPensionSplit(incomes []Income, ages []int, fractions []float64) []Income {
	out := append([]Income(nil), incomes...)
	type transfer struct {
		to   int
		db   float64
		rrif float64
	}
	moves := make([]transfer, len(incomes))
	for i := range incomes {
		fraction := fractions[i]
		if fraction <= 0 {
			continue
		}
		to := (i + 1) % len(incomes)
		moves[i] = transfer{to: to, db: incomes[i].DBPPension * fraction}
		if ages[i] >= pensionSplitAge {
			moves[i].rrif = incomes[i].RRIFWithdrawals * fraction
		}
	}
	for i, m := range moves {
		if m.db == 0 && m.rrif == 0 {
			continue
		}
		out[i].DBPPension -= m.db
		out[i].RRIFWithdrawals -= m.rrif
		out[m.to].DBPPension += m.db
		out[m.to].RRIFWithdrawals += m.rrif
	}
	return out
}
