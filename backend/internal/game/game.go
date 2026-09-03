package game

import "fmt"

// ---------------------------------------------------------------------------
// Status enums
// ---------------------------------------------------------------------------

// GameStatus represents the lifecycle stage of a game table.
type GameStatus string

const (
	StatusForming  GameStatus = "forming"
	StatusActive   GameStatus = "active"
	StatusEnded    GameStatus = "ended"
	StatusExpired  GameStatus = "expired"
	StatusCanceled GameStatus = "cancelled"
)

// RoundStatus represents the lifecycle stage of a single round.
type RoundStatus string

const (
	RoundOpen         RoundStatus = "open"
	RoundReview       RoundStatus = "review"
	RoundLocked       RoundStatus = "locked"
	RoundReadyForNext RoundStatus = "ready_for_next"
)

// AdjustmentStatus represents the lifecycle of a score adjustment.
type AdjustmentStatus string

const (
	AdjustmentPending  AdjustmentStatus = "pending"
	AdjustmentAccepted AdjustmentStatus = "accepted"
	AdjustmentRejected AdjustmentStatus = "rejected"
	AdjustmentCanceled AdjustmentStatus = "cancelled"
	AdjustmentExpired  AdjustmentStatus = "expired"
)

// AdjustmentType distinguishes补分 and退分.
type AdjustmentType string

const (
	AdjustmentSupplement AdjustmentType = "supplement" // 补分
	AdjustmentRefund     AdjustmentType = "refund"     // 退分
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// Round tracks one round's state within a game.
type Round struct {
	Number int
	Status RoundStatus
}

// Game tracks state transitions for one table. Persistence and authorization
// remain outside this domain type; the caller is responsible for loading and
// storing the game.
type Game struct {
	Status          GameStatus
	Members         int    // current number of joined players
	MembersLocked   bool   // true after first submission in round 1
	CompletedRounds int    // number of locked rounds
	CurrentRound    *Round // nil when game is ended or no round created yet
}

// ---------------------------------------------------------------------------
// Game-level state transitions (PRD §2.1)
// ---------------------------------------------------------------------------

// Start transitions a forming game to active and creates round 1.
// PRD §2.1 rules 2-3: at least 2 members, only from forming status.
func (g *Game) Start() error {
	if g.Status != StatusForming {
		return fmt.Errorf("game cannot start from %s", g.Status)
	}
	if g.Members < 2 {
		return fmt.Errorf("at least 2 members are required")
	}
	g.Status = StatusActive
	g.CurrentRound = &Round{Number: 1, Status: RoundOpen}
	return nil
}

// Cancel transitions a forming game to cancelled.
// PRD §2.1 rule 7: only forming games can be cancelled (with no completed rounds).
func (g *Game) Cancel() error {
	if g.Status != StatusForming {
		return fmt.Errorf("game cannot be cancelled from %s", g.Status)
	}
	if g.CompletedRounds > 0 {
		return fmt.Errorf("cannot cancel a game with completed rounds")
	}
	g.Status = StatusCanceled
	g.CurrentRound = nil
	return nil
}

// Expire transitions a forming game to expired.
// PRD §2.1 rule 6: forming games that exceed 24h auto-expire.
func (g *Game) Expire() error {
	if g.Status != StatusForming {
		return fmt.Errorf("game cannot expire from %s", g.Status)
	}
	g.Status = StatusExpired
	g.CurrentRound = nil
	return nil
}

// End transitions an active game to ended.
// PRD §2.2 rule 9 & §2.1 rule 8: current round must be complete, at least 1
// completed round required.
func (g *Game) End() error {
	if g.Status != StatusActive {
		return fmt.Errorf("game is not active")
	}
	if g.CurrentRound == nil {
		return fmt.Errorf("no current round")
	}
	if g.CurrentRound.Status == RoundOpen || g.CurrentRound.Status == RoundReview {
		return fmt.Errorf("current round is incomplete")
	}
	if g.CompletedRounds == 0 {
		return fmt.Errorf("game has no completed rounds")
	}
	g.Status = StatusEnded
	g.CurrentRound = nil
	return nil
}

// ---------------------------------------------------------------------------
// Round-level state transitions (PRD §2.2)
// ---------------------------------------------------------------------------

// EnterReview transitions the current round from open to review.
// PRD §2.2 rule 2: all members submitted → review.
// PRD §2.1 rule 4: first successful submission locks members; we lock here
// as a simplification since all members submitted implies at least one
// submission occurred.
func (g *Game) EnterReview() error {
	if g.Status != StatusActive || g.CurrentRound == nil {
		return fmt.Errorf("game is not active with a current round")
	}
	if g.CurrentRound.Status != RoundOpen {
		return fmt.Errorf("round is not open (status=%s)", g.CurrentRound.Status)
	}
	g.CurrentRound.Status = RoundReview
	// Lock members on first round entering review.
	if g.CurrentRound.Number == 1 {
		g.MembersLocked = true
	}
	return nil
}

// LockRound transitions the current round from review to locked/ready_for_next
// and increments completed rounds.
// PRD §2.2 rules 5-6: requires sum == 0 (caller validates zero-sum before
// calling).
func (g *Game) LockRound() error {
	if g.Status != StatusActive || g.CurrentRound == nil {
		return fmt.Errorf("game is not active with a current round")
	}
	if g.CurrentRound.Status != RoundReview {
		return fmt.Errorf("round is not in review (status=%s)", g.CurrentRound.Status)
	}
	g.CurrentRound.Status = RoundLocked
	g.CompletedRounds++
	// Immediately transition to ready_for_next so the round is "done" and
	// either next round or end can be triggered.
	g.CurrentRound.Status = RoundReadyForNext
	return nil
}

// NextRound creates the next round if the current round is ready_for_next.
// PRD §2.2 rules 7-8: only after ready_for_next, concurrent calls return same
// round (caller handles idempotency via request_id).
func (g *Game) NextRound() error {
	if g.Status != StatusActive || g.CurrentRound == nil {
		return fmt.Errorf("game is not active with a current round")
	}
	if g.CurrentRound.Status != RoundReadyForNext {
		return fmt.Errorf("next round is not ready (status=%s)", g.CurrentRound.Status)
	}
	g.CurrentRound = &Round{
		Number: g.CompletedRounds + 1,
		Status: RoundOpen,
	}
	return nil
}
