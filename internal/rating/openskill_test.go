package rating

import (
	"fmt"
	"testing"
)

func TestNewRating(t *testing.T) {
	r := NewRating()
	if r.Mu != DefaultMu {
		t.Errorf("Expected Mu=%f, got %f", DefaultMu, r.Mu)
	}
	if r.Sigma != DefaultSigma {
		t.Errorf("Expected Sigma=%f, got %f", DefaultSigma, r.Sigma)
	}
}

func TestPlackettLuceRate_WinnerGainsLoserLoses(t *testing.T) {
	winner := NewRating()
	loser := NewRating()

	newWinner, newLoser := PlackettLuceRate(winner, loser)

	if newWinner.Mu <= winner.Mu {
		t.Errorf("Winner mu should increase: before=%f, after=%f", winner.Mu, newWinner.Mu)
	}
	if newLoser.Mu >= loser.Mu {
		t.Errorf("Loser mu should decrease: before=%f, after=%f", loser.Mu, newLoser.Mu)
	}
	if newWinner.Sigma >= winner.Sigma {
		t.Errorf("Winner sigma should decrease: before=%f, after=%f", winner.Sigma, newWinner.Sigma)
	}
	if newLoser.Sigma >= loser.Sigma {
		t.Errorf("Loser sigma should decrease: before=%f, after=%f", loser.Sigma, newLoser.Sigma)
	}

	fmt.Printf("Winner: mu=%.2f -> %.2f, sigma=%.2f -> %.2f\n",
		winner.Mu, newWinner.Mu, winner.Sigma, newWinner.Sigma)
	fmt.Printf("Loser:  mu=%.2f -> %.2f, sigma=%.2f -> %.2f\n",
		loser.Mu, newLoser.Mu, loser.Sigma, newLoser.Sigma)
	fmt.Printf("Winner ordinal: %.2f -> %.2f\n", winner.Ordinal(), newWinner.Ordinal())
	fmt.Printf("Loser ordinal:  %.2f -> %.2f\n", loser.Ordinal(), newLoser.Ordinal())
}

func TestPlackettLuceRate_HigherRatedWinner(t *testing.T) {
	// If higher-rated player wins, gains should be small
	winner := NewRatingWith(1200, 200)
	loser := NewRatingWith(800, 200)

	newWinner, newLoser := PlackettLuceRate(winner, loser)

	// Higher-rated player wins - smaller gain than when equal
	fmt.Printf("Asymmetric match:\n")
	fmt.Printf("Winner (1200): mu=%.2f -> %.2f\n", winner.Mu, newWinner.Mu)
	fmt.Printf("Loser  (800):  mu=%.2f -> %.2f\n", loser.Mu, newLoser.Mu)

	if newWinner.Mu <= winner.Mu {
		t.Errorf("Winner should still gain mu")
	}
	if newLoser.Mu >= loser.Mu {
		t.Errorf("Loser should still lose mu")
	}
}

func TestOrdinal(t *testing.T) {
	r := NewRating()
	expected := DefaultMu - 3*DefaultSigma
	if r.Ordinal() != expected {
		t.Errorf("Expected ordinal=%f, got %f", expected, r.Ordinal())
	}
	fmt.Printf("Initial ordinal: %.2f (mu=%.2f, sigma=%.2f)\n", r.Ordinal(), r.Mu, r.Sigma)
}
