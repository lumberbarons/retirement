package projection

import (
	"math"

	"github.com/lumberbarons/retirement/internal/config"
)

// Retirement-smile shaping, per the governing spec §6.4: real spending holds
// flat through the early "go-go" years, declines about 1%/yr through the
// middle retirement phase (roughly the 70s and 80s for a retiree at 60), then
// holds flat. Late-life care is a dated lumpy stream, not part of this shape.
const (
	smileGoGoYears     = 10
	smileDeclineYears  = 15
	smileAnnualDecline = 0.01
)

// nominalSpending returns the household's nominal spending target for the
// year. The configured target is stated in today's dollars; it is shaped,
// has any lumpy streams added, is scaled by the survivor factor once one
// spouse has died, and is then inflated at the spending rate — deliberately
// separate from tax-bracket indexation (§6.5). Before the first retirement
// year there is no spending target, so dated streams are not funded either.
func (s *State) nominalSpending(h *config.Household) float64 {
	first := s.firstRetirementYear()
	if s.Year < first {
		return 0
	}
	real := h.Spending.TargetTodayDollars * s.shapeFactor(h)
	if s.survivors() == 1 {
		// The survivor factor scales the household's lifestyle target but
		// not the dated streams: a roof or a care cost is not 30% cheaper
		// because there is one person left.
		real *= h.Spending.SurvivorFactor
	}
	real += s.lumpyTodayDollars(h, s.Year)
	nominal := real * math.Pow(1+h.Spending.Inflation, float64(s.Year-h.BaseYear))
	if s.Year == first {
		// The engine models whole years, so the first retirement year is
		// approximated as starting mid-year.
		nominal *= midYearFraction
	}
	return RoundCents(nominal)
}

// shapeFactor is the real spending multiplier for the year: flat mode is 1,
// smile mode follows the retirement phase curve.
func (s *State) shapeFactor(h *config.Household) float64 {
	if h.Spending.Mode == "smile" {
		return smileFactor(s.Year - s.firstRetirementYear())
	}
	return 1
}

// smileFactor returns the real spending multiplier for a year that is
// yearsRetired after the first retirement year.
func smileFactor(yearsRetired int) float64 {
	if yearsRetired < smileGoGoYears {
		return 1
	}
	decline := float64(yearsRetired - smileGoGoYears + 1)
	if decline > smileDeclineYears {
		decline = smileDeclineYears
	}
	return math.Pow(1-smileAnnualDecline, decline)
}

// lumpyTodayDollars sums the dated streams that fall in the year, in today's
// dollars. They are inflated with the base target, not separately.
func (s *State) lumpyTodayDollars(h *config.Household, year int) float64 {
	total := 0.0
	for _, item := range h.Spending.Lumpy {
		if lumpyApplies(item, year) {
			total += item.AmountTodayDollars
		}
	}
	return total
}

// lumpyApplies reports whether an item falls in the year. An item with no
// end year is a one-off at its start year; otherwise it runs through the
// inclusive range, at every step when every_years is 0 or 1, or every Nth
// year from the start otherwise.
func lumpyApplies(item config.LumpyItem, year int) bool {
	if year < item.StartYear {
		return false
	}
	end := item.EndYear
	if end == 0 {
		end = item.StartYear
	}
	if year > end {
		return false
	}
	if item.EveryYears <= 1 {
		return true
	}
	return (year-item.StartYear)%item.EveryYears == 0
}

// survivors counts the spouses still alive. Deaths are applied at the end of
// the death year, so the survivor factor takes effect the year after.
func (s *State) survivors() int {
	alive := 0
	for _, p := range s.People {
		if p.Alive {
			alive++
		}
	}
	return alive
}
