package scoring

import "testing"

func TestValidate(t *testing.T) {
	seats := []int{1, 2, 3}
	tests := []struct {
		name   string
		scores Scores
		passed bool
		sum    int64
	}{
		{"zero sum", Scores{1: 16, 2: -8, 3: -8}, true, 0},
		{"non zero", Scores{1: 16, 2: -8, 3: -7}, false, 1},
		{"zero scores", Scores{1: 0, 2: 0, 3: 0}, true, 0},
		{"missing score", Scores{1: 16, 2: -16}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Validate(tt.scores, seats)
			if got.Passed != tt.passed || got.Sum != tt.sum {
				t.Fatalf("Validate() = %+v, want passed=%v sum=%d", got, tt.passed, tt.sum)
			}
		})
	}
}

func TestTransfer(t *testing.T) {
	totals := map[int]int64{1: 16, 2: -16}
	if err := Transfer(totals, 1, 2, 8); err != nil {
		t.Fatal(err)
	}
	if totals[1] != 8 || totals[2] != -8 {
		t.Fatalf("totals = %#v, want 1:8, 2:-8", totals)
	}
	if err := Transfer(totals, 1, 1, 1); err == nil {
		t.Fatal("Transfer() accepted same seat")
	}
	if err := Transfer(totals, 1, 2, 0); err == nil {
		t.Fatal("Transfer() accepted zero amount")
	}
}

func TestRank(t *testing.T) {
	got := Rank(map[int]int64{1: 8, 2: 8, 3: -16})
	want := []Standing{{Seat: 1, Score: 8, Rank: 1}, {Seat: 2, Score: 8, Rank: 1}, {Seat: 3, Score: -16, Rank: 3}}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Rank()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
