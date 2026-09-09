package rank

import "testing"

func TestSettle(t *testing.T) {
	// 四人局：A +45 胜，B +12 第二平，C -25 第三平，D -32 负
	scores := map[int64]int{1: 45, 2: 12, 3: -25, 4: -32}
	streaks := map[int64]int{1: 0, 2: 0, 3: 0, 4: 2}
	out := Settle(scores, streaks)

	if out[1].Result != "win" || out[1].StarsDelta != 1 || out[1].StreakAfter != 1 {
		t.Fatalf("A outcome = %+v, want win +1", out[1])
	}
	if out[2].Result != "draw" || out[2].StarsDelta != 0 || out[2].StreakAfter != 0 {
		t.Fatalf("B outcome = %+v, want draw 0", out[2])
	}
	if out[3].Result != "draw" || out[3].StarsDelta != 0 {
		t.Fatalf("C outcome = %+v, want draw 0", out[3])
	}
	if out[4].Result != "lose" || out[4].StarsDelta != -1 || out[4].StreakAfter != 0 {
		t.Fatalf("D outcome = %+v, want lose -1", out[4])
	}
}

func TestSettleStreakBonus(t *testing.T) {
	// 原连胜 2，再胜 → 3 连，额外 +1（共 +2）
	out := Settle(map[int64]int{1: 10, 2: -10}, map[int64]int{1: 2, 2: 0})
	if out[1].StarsDelta != 2 || out[1].BonusStars != 1 || out[1].StreakAfter != 3 {
		t.Fatalf("3连 outcome = %+v, want +2 (bonus 1), streak 3", out[1])
	}

	// 原连胜 5，再胜 → 6 连，额外 +2（共 +3）
	out = Settle(map[int64]int{1: 10, 2: -10}, map[int64]int{1: 5, 2: 0})
	if out[1].StarsDelta != 3 || out[1].BonusStars != 2 || out[1].StreakAfter != 6 {
		t.Fatalf("6连 outcome = %+v, want +3 (bonus 2), streak 6", out[1])
	}

	// 连胜 2 未达奖励线 → 只有基础 +1
	out = Settle(map[int64]int{1: 10, 2: -10}, map[int64]int{1: 1, 2: 0})
	if out[1].StarsDelta != 1 || out[1].BonusStars != 0 {
		t.Fatalf("2连 outcome = %+v, want +1 no bonus", out[1])
	}

	// 平局清空连胜
	out = Settle(map[int64]int{1: 0, 2: 0}, map[int64]int{1: 4, 2: 0})
	if out[1].Result != "draw" || out[1].StreakAfter != 0 {
		t.Fatalf("draw outcome = %+v, want streak reset", out[1])
	}

	// 全员同分（全场流局）：平，星级不变
	out = Settle(map[int64]int{1: 0, 2: 0, 3: 0, 4: 0}, nil)
	for id := int64(1); id <= 4; id++ {
		if out[id].Result != "draw" || out[id].StarsDelta != 0 {
			t.Fatalf("all-tie p%d outcome = %+v, want draw 0", id, out[id])
		}
	}

	// 并列最高：都是胜
	out = Settle(map[int64]int{1: 20, 2: 20, 3: -20, 4: -20}, nil)
	if out[1].Result != "win" || out[2].Result != "win" || out[3].Result != "lose" || out[4].Result != "lose" {
		t.Fatalf("tie outcome = %+v/%+v/%+v/%+v", out[1], out[2], out[3], out[4])
	}
}

func TestInfoFromStars(t *testing.T) {
	cases := []struct {
		stars  int
		tier   int
		inTier int
		need   int
		roman  string
	}{
		{0, 1, 0, 3, "初"},
		{2, 1, 2, 3, "II"},
		{3, 2, 0, 3, "初"},  // 八品
		{6, 3, 0, 4, "初"},  // 六品
		{9, 3, 3, 4, "III"}, // 六品 III
		{10, 4, 0, 4, "初"}, // 四品
		{14, 5, 0, 6, "初"}, // 二品
		{20, 6, 0, 0, "初"},   // 至尊
		{99, 6, 79, 0, "79"}, // 至尊无上限，段内星 = 总星 - 20
	}
	for _, c := range cases {
		info := InfoFromStars(c.stars)
		if info.TierIndex != c.tier || info.StarsInTier != c.inTier || info.StarsNeeded != c.need {
			t.Fatalf("InfoFromStars(%d) = tier %d %d/%d, want tier %d %d/%d",
				c.stars, info.TierIndex, info.StarsInTier, info.StarsNeeded, c.tier, c.inTier, c.need)
		}
	}
}
