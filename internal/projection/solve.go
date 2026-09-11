package projection

import (
	"math"

	"github.com/lumberbarons/retirement/internal/config"
	"github.com/lumberbarons/retirement/internal/tax"
)

// solveTolerance is the width of the final bisection bracket: half a cent,
// well inside the governing spec's $1 tolerance on net spending.
const solveTolerance = 0.005

// maxSolveIterations caps the bisection at the spec's ~50 iterations.
const maxSolveIterations = 50

// NetIncomeFunc reports the after-tax household spending produced by a trial
// total discretionary withdrawal. More gross withdrawal must never produce
// less net spending — true even inside the OAS clawback zone, where the
// effective marginal rate exceeds 60% — so the solve can bisect safely.
type NetIncomeFunc func(discretionary float64) float64

// SolveGross returns the discretionary withdrawal that brings net spending
// to target, following the §5.1 baseline priority order. If the target is
// already met without a withdrawal, it returns 0; if even the available
// capacity cannot reach the target, it returns capacity and the plan falls
// short. The result is not rounded; the caller commits it to cents.
func SolveGross(target, capacity float64, net NetIncomeFunc) float64 {
	if capacity <= 0 {
		return 0
	}
	if net(0) >= target {
		return 0
	}
	if net(capacity) < target {
		return capacity
	}
	lo, hi := 0.0, capacity
	for i := 0; i < maxSolveIterations && hi-lo > solveTolerance; i++ {
		mid := (lo + hi) / 2
		if net(mid) >= target {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// withdrawalTiers is the baseline priority order (§5.1): non-registered
// first, then registered RRSP/RRIF, TFSA last.
var withdrawalTiers = [][]config.AccountType{
	{config.AccountNonRegistered},
	{config.AccountRRSP, config.AccountRRIF},
	{config.AccountTFSA},
}

// withdrawalCapacity is the total balance available for discretionary
// withdrawals this year.
func (s *State) withdrawalCapacity() float64 {
	total := 0.0
	for _, a := range s.Accounts {
		total += a.Balance
	}
	return total
}

// allocate returns, without mutating state, how much to withdraw from each
// account to reach amount. Tiers are drained in priority order, and accounts
// within a tier are drawn pro-rata by balance — per spec 001, withdrawals
// allocate proportionally across spouses rather than front-loading config
// order.
func (s *State) allocate(amount float64) []float64 {
	alloc := make([]float64, len(s.Accounts))
	remaining := amount
	for _, tier := range withdrawalTiers {
		if remaining <= 0 {
			break
		}
		tierTotal := 0.0
		for _, a := range s.Accounts {
			if inTier(a.Type, tier) {
				tierTotal += a.Balance
			}
		}
		if tierTotal <= 0 {
			continue
		}
		draw := math.Min(remaining, tierTotal)
		for i, a := range s.Accounts {
			if inTier(a.Type, tier) {
				alloc[i] = draw * a.Balance / tierTotal
			}
		}
		remaining -= draw
	}
	return alloc
}

func inTier(t config.AccountType, tier []config.AccountType) bool {
	for _, candidate := range tier {
		if candidate == t {
			return true
		}
	}
	return false
}

// applyWithdrawals records the allocation, adds each withdrawal's taxable
// portion to the year's income, and removes it from balances. A
// non-registered withdrawal reduces ACB proportionally; the sub-cent
// floating-point residue a proportional split can leave behind is clamped.
func (s *State) applyWithdrawals(alloc []float64, incomes []tax.Income, res *YearResult) {
	for i, w := range alloc {
		if w <= 0 {
			continue
		}
		a := s.Accounts[i]
		s.addWithdrawalIncome(incomes, i, w)
		if a.Type == config.AccountNonRegistered && a.Balance > 0 {
			ratio := a.ACB / a.Balance
			if ratio > 1 {
				ratio = 1
			}
			a.ACB = math.Max(0, a.ACB-w*ratio)
		}
		a.Balance = math.Max(0, a.Balance-w)
		res.Accounts[i].Withdrawal += w
		res.Withdrawals += w
	}
}
