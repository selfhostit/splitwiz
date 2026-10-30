package ledger

import (
	"errors"
	"math"
	"time"
)

// How often a recurring expense repeats.
const (
	EveryWeek  = "week"
	EveryMonth = "month"
	EveryYear  = "year"
)

var ErrEvery = errors.New("repeat must be week, month or year")

// NextDate returns the occurrence after `from`. Monthly and yearly repeats
// stay on anchorDay (the day of the month the series started on), falling
// back to the month's last day when it is shorter: a series started on the
// 31st runs Jan 31, Feb 28, Mar 31.
func NextDate(from time.Time, every string, anchorDay int) (time.Time, error) {
	if anchorDay < 1 || anchorDay > 31 {
		anchorDay = from.Day()
	}

	switch every {
	case EveryWeek:
		return from.AddDate(0, 0, 7), nil
	case EveryMonth:
		return onDay(from.Year(), from.Month()+1, anchorDay, from), nil
	case EveryYear:
		return onDay(from.Year()+1, from.Month(), anchorDay, from), nil
	}
	return time.Time{}, ErrEvery
}

func onDay(year int, month time.Month, day int, like time.Time) time.Time {
	// normalise month overflow (13 -> January next year) first
	first := time.Date(year, month, 1, like.Hour(), like.Minute(), like.Second(), 0, like.Location())
	last := first.AddDate(0, 1, -1).Day()
	if day > last {
		day = last
	}
	return first.AddDate(0, 0, day-1)
}

// AmountChange is a scheduled new amount for a recurring expense: every
// occurrence on or after From (YYYY-MM-DD) costs Cents, until a later change.
type AmountChange struct {
	From  string `json:"from"`
	Cents int64  `json:"amount_cents"`
}

// AmountOn is what an occurrence on `day` (YYYY-MM-DD) costs: the latest
// change that has started by then, or `base` before the first one.
func AmountOn(base int64, changes []AmountChange, day string) int64 {
	amount, from := base, ""
	for _, c := range changes {
		if c.From <= day && c.From >= from {
			amount, from = c.Cents, c.From
		}
	}
	return amount
}

// ScaleShare keeps an exact split's share in proportion when the amount
// changes: a €20 share of €50 becomes €24 of €60.
func ScaleShare(share, base, amount int64) int64 {
	if base <= 0 || amount == base {
		return share
	}
	scaled := int64(math.Round(float64(share) * float64(amount) / float64(base)))
	if scaled > amount {
		return amount
	}
	return scaled
}
