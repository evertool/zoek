package scoring

import "fmt"

// Scores contains one player's score for each seat in a round.
type Scores map[int]int64

// ValidationResult describes whether scores can lock a round.
type ValidationResult struct {
	Passed bool
	Sum    int64
	Error  string
}

// Validate checks that every member submitted an integer score and that the
// round is zero-sum.
func Validate(scores Scores, memberSeats []int) ValidationResult {
	var sum int64
	for _, seat := range memberSeats {
		score, ok := scores[seat]
		if !ok {
			return ValidationResult{Error: fmt.Sprintf("seat %d has not submitted", seat)}
		}
		sum += score
	}
	if sum != 0 {
		return ValidationResult{
			Sum:   sum,
			Error: fmt.Sprintf("score sum is %d, want 0", sum),
		}
	}
	return ValidationResult{Passed: true}
}

// Transfer applies a confirmed correction without changing table total.
func Transfer(totals map[int]int64, fromSeat, toSeat int, amount int64) error {
	if fromSeat == toSeat {
		return fmt.Errorf("from and to seats must differ")
	}
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	totals[fromSeat] -= amount
	totals[toSeat] += amount
	return nil
}

// Rank returns seats ordered by total score descending. Equal scores share a
// rank using competition ranking: 1, 1, 3.
type Standing struct {
	Seat  int
	Score int64
	Rank  int
}

func Rank(totals map[int]int64) []Standing {
	result := make([]Standing, 0, len(totals))
	for seat, score := range totals {
		result = append(result, Standing{Seat: seat, Score: score})
	}
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && (result[j].Score > result[j-1].Score ||
			(result[j].Score == result[j-1].Score && result[j].Seat < result[j-1].Seat)); j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	for i := range result {
		if i == 0 || result[i].Score != result[i-1].Score {
			result[i].Rank = i + 1
		} else {
			result[i].Rank = result[i-1].Rank
		}
	}
	return result
}
