package game

import "testing"

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newFormingGame creates a forming game with the given member count.
func newFormingGame(members int) *Game {
	return &Game{Status: StatusForming, Members: members}
}

// newActiveGame creates an active game with round 1 open.
func newActiveGame(members int) *Game {
	g := newFormingGame(members)
	_ = g.Start()
	return g
}

// ---------------------------------------------------------------------------
// Start (PRD §2.1 rules 2-3)
// ---------------------------------------------------------------------------

func TestStart(t *testing.T) {
	t.Run("success with 2 members", func(t *testing.T) {
		g := newFormingGame(2)
		if err := g.Start(); err != nil {
			t.Fatalf("Start() error: %v", err)
		}
		if g.Status != StatusActive {
			t.Fatalf("status = %s, want active", g.Status)
		}
		if g.CurrentRound == nil || g.CurrentRound.Number != 1 || g.CurrentRound.Status != RoundOpen {
			t.Fatalf("current round = %+v, want round 1 open", g.CurrentRound)
		}
	})

	t.Run("success with 4 members", func(t *testing.T) {
		g := newFormingGame(4)
		if err := g.Start(); err != nil {
			t.Fatalf("Start() error: %v", err)
		}
		if g.Status != StatusActive {
			t.Fatalf("status = %s, want active", g.Status)
		}
	})

	t.Run("fail with 1 member", func(t *testing.T) {
		g := newFormingGame(1)
		if err := g.Start(); err == nil {
			t.Fatal("Start() should fail with < 2 members")
		}
		if g.Status != StatusForming {
			t.Fatalf("status = %s, want still forming", g.Status)
		}
	})

	t.Run("fail from active", func(t *testing.T) {
		g := newActiveGame(3)
		if err := g.Start(); err == nil {
			t.Fatal("Start() should fail from active")
		}
	})

	t.Run("fail from ended", func(t *testing.T) {
		g := &Game{Status: StatusEnded}
		if err := g.Start(); err == nil {
			t.Fatal("Start() should fail from ended")
		}
	})
}

// ---------------------------------------------------------------------------
// Cancel (PRD §2.1 rule 7)
// ---------------------------------------------------------------------------

func TestCancel(t *testing.T) {
	t.Run("success from forming", func(t *testing.T) {
		g := newFormingGame(2)
		if err := g.Cancel(); err != nil {
			t.Fatalf("Cancel() error: %v", err)
		}
		if g.Status != StatusCanceled {
			t.Fatalf("status = %s, want cancelled", g.Status)
		}
		if g.CurrentRound != nil {
			t.Fatalf("current round should be nil, got %+v", g.CurrentRound)
		}
	})

	t.Run("fail from active", func(t *testing.T) {
		g := newActiveGame(3)
		if err := g.Cancel(); err == nil {
			t.Fatal("Cancel() should fail from active")
		}
	})

	t.Run("fail from ended", func(t *testing.T) {
		g := &Game{Status: StatusEnded}
		if err := g.Cancel(); err == nil {
			t.Fatal("Cancel() should fail from ended")
		}
	})
}

// ---------------------------------------------------------------------------
// Expire (PRD §2.1 rule 6)
// ---------------------------------------------------------------------------

func TestExpire(t *testing.T) {
	t.Run("success from forming", func(t *testing.T) {
		g := newFormingGame(2)
		if err := g.Expire(); err != nil {
			t.Fatalf("Expire() error: %v", err)
		}
		if g.Status != StatusExpired {
			t.Fatalf("status = %s, want expired", g.Status)
		}
		if g.CurrentRound != nil {
			t.Fatalf("current round should be nil, got %+v", g.CurrentRound)
		}
	})

	t.Run("fail from active", func(t *testing.T) {
		g := newActiveGame(3)
		if err := g.Expire(); err == nil {
			t.Fatal("Expire() should fail from active")
		}
	})
}

// ---------------------------------------------------------------------------
// EnterReview (PRD §2.2 rule 2, §2.1 rule 4)
// ---------------------------------------------------------------------------

func TestEnterReview(t *testing.T) {
	t.Run("success from open", func(t *testing.T) {
		g := newActiveGame(3)
		if err := g.EnterReview(); err != nil {
			t.Fatalf("EnterReview() error: %v", err)
		}
		if g.CurrentRound.Status != RoundReview {
			t.Fatalf("status = %s, want review", g.CurrentRound.Status)
		}
	})

	t.Run("locks members on round 1", func(t *testing.T) {
		g := newActiveGame(3)
		if g.MembersLocked {
			t.Fatal("members should not be locked before first review")
		}
		_ = g.EnterReview()
		if !g.MembersLocked {
			t.Fatal("members should be locked after round 1 enters review")
		}
	})

	t.Run("does not re-lock on round 2", func(t *testing.T) {
		g := newActiveGame(3)
		_ = g.EnterReview()
		_ = g.LockRound()
		_ = g.NextRound()
		if g.CurrentRound.Number != 2 {
			t.Fatalf("round number = %d, want 2", g.CurrentRound.Number)
		}
		// MembersLocked should already be true from round 1.
		if !g.MembersLocked {
			t.Fatal("members should remain locked")
		}
		_ = g.EnterReview()
		if !g.MembersLocked {
			t.Fatal("members should still be locked after round 2 review")
		}
	})

	t.Run("fail from review", func(t *testing.T) {
		g := newActiveGame(3)
		_ = g.EnterReview()
		if err := g.EnterReview(); err == nil {
			t.Fatal("EnterReview() should fail from review")
		}
	})

	t.Run("fail from forming", func(t *testing.T) {
		g := newFormingGame(3)
		if err := g.EnterReview(); err == nil {
			t.Fatal("EnterReview() should fail from forming")
		}
	})
}

// ---------------------------------------------------------------------------
// LockRound (PRD §2.2 rules 5-6)
// ---------------------------------------------------------------------------

func TestLockRound(t *testing.T) {
	t.Run("success from review", func(t *testing.T) {
		g := newActiveGame(3)
		_ = g.EnterReview()
		if err := g.LockRound(); err != nil {
			t.Fatalf("LockRound() error: %v", err)
		}
		if g.CurrentRound.Status != RoundReadyForNext {
			t.Fatalf("status = %s, want ready_for_next", g.CurrentRound.Status)
		}
		if g.CompletedRounds != 1 {
			t.Fatalf("completed rounds = %d, want 1", g.CompletedRounds)
		}
	})

	t.Run("fail from open", func(t *testing.T) {
		g := newActiveGame(3)
		if err := g.LockRound(); err == nil {
			t.Fatal("LockRound() should fail from open")
		}
	})

	t.Run("fail from ready_for_next", func(t *testing.T) {
		g := newActiveGame(3)
		_ = g.EnterReview()
		_ = g.LockRound()
		if err := g.LockRound(); err == nil {
			t.Fatal("LockRound() should fail from ready_for_next")
		}
	})
}

// ---------------------------------------------------------------------------
// NextRound (PRD §2.2 rules 7-8)
// ---------------------------------------------------------------------------

func TestNextRound(t *testing.T) {
	t.Run("success after lock", func(t *testing.T) {
		g := newActiveGame(3)
		_ = g.EnterReview()
		_ = g.LockRound()
		if err := g.NextRound(); err != nil {
			t.Fatalf("NextRound() error: %v", err)
		}
		if g.CurrentRound.Number != 2 {
			t.Fatalf("round number = %d, want 2", g.CurrentRound.Number)
		}
		if g.CurrentRound.Status != RoundOpen {
			t.Fatalf("status = %s, want open", g.CurrentRound.Status)
		}
	})

	t.Run("fail from open", func(t *testing.T) {
		g := newActiveGame(3)
		if err := g.NextRound(); err == nil {
			t.Fatal("NextRound() should fail from open")
		}
	})

	t.Run("fail from review", func(t *testing.T) {
		g := newActiveGame(3)
		_ = g.EnterReview()
		if err := g.NextRound(); err == nil {
			t.Fatal("NextRound() should fail from review")
		}
	})

	t.Run("succeeds on round 3", func(t *testing.T) {
		g := newActiveGame(3)
		// Round 1
		_ = g.EnterReview()
		_ = g.LockRound()
		_ = g.NextRound()
		// Round 2
		_ = g.EnterReview()
		_ = g.LockRound()
		_ = g.NextRound()
		if g.CurrentRound.Number != 3 {
			t.Fatalf("round number = %d, want 3", g.CurrentRound.Number)
		}
		if g.CompletedRounds != 2 {
			t.Fatalf("completed rounds = %d, want 2", g.CompletedRounds)
		}
	})
}

// ---------------------------------------------------------------------------
// End (PRD §2.1 rule 8, §2.2 rule 9)
// ---------------------------------------------------------------------------

func TestEnd(t *testing.T) {
	t.Run("success after round completed", func(t *testing.T) {
		g := newActiveGame(3)
		_ = g.EnterReview()
		_ = g.LockRound()
		if err := g.End(); err != nil {
			t.Fatalf("End() error: %v", err)
		}
		if g.Status != StatusEnded {
			t.Fatalf("status = %s, want ended", g.Status)
		}
		if g.CurrentRound != nil {
			t.Fatalf("current round should be nil, got %+v", g.CurrentRound)
		}
	})

	t.Run("fail from forming", func(t *testing.T) {
		g := newFormingGame(3)
		if err := g.End(); err == nil {
			t.Fatal("End() should fail from forming")
		}
	})

	t.Run("fail with open round", func(t *testing.T) {
		g := newActiveGame(3)
		if err := g.End(); err == nil {
			t.Fatal("End() should fail with open round")
		}
	})

	t.Run("fail with review round", func(t *testing.T) {
		g := newActiveGame(3)
		_ = g.EnterReview()
		if err := g.End(); err == nil {
			t.Fatal("End() should fail with review round")
		}
	})

	t.Run("fail with no completed rounds", func(t *testing.T) {
		g := &Game{
			Status:       StatusActive,
			CurrentRound: &Round{Number: 1, Status: RoundReadyForNext},
		}
		if err := g.End(); err == nil {
			t.Fatal("End() should fail with no completed rounds")
		}
	})

	t.Run("fail from ended", func(t *testing.T) {
		g := &Game{Status: StatusEnded}
		if err := g.End(); err == nil {
			t.Fatal("End() should fail from ended")
		}
	})
}

// ---------------------------------------------------------------------------
// Full lifecycle walkthrough (PRD §2.2 flow)
// ---------------------------------------------------------------------------

func TestFullLifecycle(t *testing.T) {
	g := newFormingGame(4)

	// Start → active + round 1 open
	if err := g.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if g.Status != StatusActive || g.CurrentRound.Number != 1 {
		t.Fatalf("after Start: %+v", g)
	}

	// All submit → review
	if err := g.EnterReview(); err != nil {
		t.Fatalf("EnterReview: %v", err)
	}
	if !g.MembersLocked {
		t.Fatal("members should be locked after round 1 review")
	}

	// Lock → ready_for_next, completed = 1
	if err := g.LockRound(); err != nil {
		t.Fatalf("LockRound: %v", err)
	}
	if g.CompletedRounds != 1 {
		t.Fatalf("completed = %d, want 1", g.CompletedRounds)
	}

	// Next → round 2 open
	if err := g.NextRound(); err != nil {
		t.Fatalf("NextRound: %v", err)
	}
	if g.CurrentRound.Number != 2 {
		t.Fatalf("round = %d, want 2", g.CurrentRound.Number)
	}

	// Round 2 lifecycle
	_ = g.EnterReview()
	_ = g.LockRound()
	_ = g.NextRound()
	if g.CurrentRound.Number != 3 || g.CompletedRounds != 2 {
		t.Fatalf("after round 2: round=%d completed=%d", g.CurrentRound.Number, g.CompletedRounds)
	}

	// Round 3 — end without creating next
	_ = g.EnterReview()
	_ = g.LockRound()
	// Now ready_for_next with 3 completed rounds
	if err := g.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
	if g.Status != StatusEnded {
		t.Fatalf("status = %s, want ended", g.Status)
	}
	if g.CurrentRound != nil {
		t.Fatalf("current round should be nil after end")
	}
	if g.CompletedRounds != 3 {
		t.Fatalf("completed = %d, want 3", g.CompletedRounds)
	}
}
