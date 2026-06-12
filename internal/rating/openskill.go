// Package rating implements the OpenSkill rating system using the Plackett-Luce model.
// This is a faithful Go port of the openskill.py library's PlackettLuce algorithm.
// Reference: https://openskill.me/en/stable/manual.html
// Algorithm: "Algorithm 4 by Weng and Lin (2011)"
package rating

import (
	"math"
)

const (
	// DefaultMu is the default mean skill value (set to 1000 as requested).
	DefaultMu = 1000.0

	// DefaultSigma = DefaultMu / 3
	DefaultSigma = DefaultMu / 3.0

	// Beta = DefaultSigma / 2 (= DefaultMu / 6)
	Beta = DefaultSigma / 2.0

	// Tau is the additive dynamics factor (= DefaultMu / 300 scaled accordingly)
	// Original: tau = 25/300 ≈ 0.0833. Scaled: tau = 1000/300 ≈ 3.333
	Tau = DefaultMu / 300.0

	// Kappa prevents variance from going negative
	Kappa = 0.0001
)

// Rating represents a player's skill rating.
type Rating struct {
	Mu    float64 `json:"mu"`
	Sigma float64 `json:"sigma"`
}

// NewRating creates a new Rating with default values (mu=1000, sigma=333.33).
func NewRating() Rating {
	return Rating{
		Mu:    DefaultMu,
		Sigma: DefaultSigma,
	}
}

// NewRatingWith creates a new Rating with specified values.
func NewRatingWith(mu, sigma float64) Rating {
	return Rating{Mu: mu, Sigma: sigma}
}

// Ordinal returns mu - 3*sigma, a conservative lower bound (99.7% confidence).
func (r Rating) Ordinal() float64 {
	return r.Mu - 3*r.Sigma
}

// DisplayRating returns an integer display rating (ordinal rounded).
func (r Rating) DisplayRating() int {
	ord := r.Ordinal()
	if ord < 0 {
		return 0
	}
	return int(math.Round(ord))
}

// teamRating holds the aggregated rating for a team (single player in 1v1).
type teamRating struct {
	mu           float64
	sigmaSquared float64
	rank         int
}

// phiMajor is the standard normal CDF (Φ).
func phiMajor(x float64) float64 {
	return 0.5 * (1.0 + math.Erf(x/math.Sqrt2))
}

// calcC computes the collective team sigma (c).
// c = sqrt( sum_i(sigma_i^2 + beta^2) )
func calcC(teams []teamRating) float64 {
	betaSq := Beta * Beta
	sum := 0.0
	for _, t := range teams {
		sum += t.sigmaSquared + betaSq
	}
	return math.Sqrt(sum)
}

// sumQ computes sum_q for each team.
// sum_q[q] = sum over teams s where rank(s) >= rank(q) of exp(mu_s / c)
func sumQ(teams []teamRating, c float64) []float64 {
	result := make([]float64, len(teams))
	for q, tq := range teams {
		s := 0.0
		for _, ti := range teams {
			if ti.rank >= tq.rank {
				s += math.Exp(ti.mu / c)
			}
		}
		result[q] = s
	}
	return result
}

// countA computes a[q] = number of teams with same rank as team q.
func countA(teams []teamRating) []int {
	result := make([]int, len(teams))
	for i, ti := range teams {
		count := 0
		for _, tj := range teams {
			if tj.rank == ti.rank {
				count++
			}
		}
		result[i] = count
	}
	return result
}

// gamma is the default gamma function: sqrt(sigma^2) / c
func gamma(c float64, sigmaSquared float64) float64 {
	return math.Sqrt(sigmaSquared) / c
}

// PlackettLuceRate computes updated ratings for a 1v1 match.
// winner is rank 1 (first place), loser is rank 2 (second place).
// Returns updated (newWinner, newLoser).
func PlackettLuceRate(winner, loser Rating) (newWinner, newLoser Rating) {
	// Apply tau (additive dynamics to sigma before calculation)
	tauSq := Tau * Tau
	w := Rating{
		Mu:    winner.Mu,
		Sigma: math.Sqrt(winner.Sigma*winner.Sigma + tauSq),
	}
	l := Rating{
		Mu:    loser.Mu,
		Sigma: math.Sqrt(loser.Sigma*loser.Sigma + tauSq),
	}

	// Build team ratings
	teams := []teamRating{
		{mu: w.Mu, sigmaSquared: w.Sigma * w.Sigma, rank: 1},
		{mu: l.Mu, sigmaSquared: l.Sigma * l.Sigma, rank: 2},
	}

	c := calcC(teams)
	sQ := sumQ(teams, c)
	a := countA(teams)

	// Compute omega and delta for each team
	type update struct {
		omega float64
		delta float64
	}
	updates := make([]update, 2)

	for i, ti := range teams {
		iMuOverC := math.Exp(ti.mu / c)
		omega := 0.0
		delta := 0.0

		for q, tq := range teams {
			iMuOverCOverSumQ := iMuOverC / sQ[q]
			if tq.rank <= ti.rank {
				delta += iMuOverCOverSumQ * (1.0 - iMuOverCOverSumQ) / float64(a[q])
				if q == i {
					omega += (1.0 - iMuOverCOverSumQ) / float64(a[q])
				} else {
					omega -= iMuOverCOverSumQ / float64(a[q])
				}
			}
		}

		omega *= ti.sigmaSquared / c
		delta *= ti.sigmaSquared / (c * c)

		// Apply gamma
		gammaVal := gamma(c, ti.sigmaSquared)
		delta *= gammaVal

		updates[i] = update{omega: omega, delta: delta}
	}

	// Update winner (index 0)
	wOmega := updates[0].omega
	wDelta := updates[0].delta
	wSigmaSq := w.Sigma * w.Sigma
	var wNewMu, wNewSigma float64
	if wOmega >= 0 {
		wNewMu = w.Mu + (wSigmaSq/teams[0].sigmaSquared)*wOmega
		wNewSigma = w.Sigma * math.Sqrt(math.Max(1.0-(wSigmaSq/teams[0].sigmaSquared)*wDelta, Kappa))
	} else {
		wNewMu = w.Mu + (wSigmaSq/teams[0].sigmaSquared)*wOmega
		wNewSigma = w.Sigma * math.Sqrt(math.Max(1.0-(wSigmaSq/teams[0].sigmaSquared)*wDelta, Kappa))
	}

	// Update loser (index 1)
	lOmega := updates[1].omega
	lDelta := updates[1].delta
	lSigmaSq := l.Sigma * l.Sigma
	var lNewMu, lNewSigma float64
	if lOmega >= 0 {
		lNewMu = l.Mu + (lSigmaSq/teams[1].sigmaSquared)*lOmega
		lNewSigma = l.Sigma * math.Sqrt(math.Max(1.0-(lSigmaSq/teams[1].sigmaSquared)*lDelta, Kappa))
	} else {
		lNewMu = l.Mu + (lSigmaSq/teams[1].sigmaSquared)*lOmega
		lNewSigma = l.Sigma * math.Sqrt(math.Max(1.0-(lSigmaSq/teams[1].sigmaSquared)*lDelta, Kappa))
	}

	newWinner = Rating{Mu: wNewMu, Sigma: wNewSigma}
	newLoser = Rating{Mu: lNewMu, Sigma: lNewSigma}
	return
}

// PredictWin returns the probability that p1 wins against p2.
// Uses the 2-player formula from openskill.py predict_win.
func PredictWin(p1, p2 Rating) float64 {
	betaSq := Beta * Beta
	p1SigSq := p1.Sigma * p1.Sigma
	p2SigSq := p2.Sigma * p2.Sigma
	denom := math.Sqrt(2*betaSq + p1SigSq + p2SigSq)
	return phiMajor((p1.Mu - p2.Mu) / denom)
}
