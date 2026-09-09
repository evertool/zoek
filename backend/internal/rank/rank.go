// Package rank — 排位段位与星级结算（仅 4 人排位局适用）。
//
// 规则（PRD paiwei 设计稿）：
//   - 一场 4 人局结束，按总得分排名：最高分者胜 +1 星，最低分者输 -1 星，其余为平不扣星；
//   - 连胜奖励：胜场后连胜达到 3~5 场额外 +1 星，达到 6 场及以上额外 +2 星（平/输清零连胜）；
//   - 保底保护：星星最低为 0，九品 0 星时输了不掉星；
//   - 段位：六品雀位，九品→八品→六品→四品→二品→至尊，星满自动升段，至尊无上限。
package rank

import "fmt"

// Tier 描述一个段位。
type Tier struct {
	Index       int    // 1-6，1 最低
	Name        string // 全名，如 九品·初入雀境
	Short       string // 短名，如 九品
	Grade       string // 级别，如 青铜级
	Tile        string // 徽标用字：雀/發/中/神
	StarsNeeded int    // 本段集满几星升下一段，0 表示无上限（至尊）
}

// Tiers 段位表，下标 0 为最低段。
var Tiers = []Tier{
	{1, "九品·初入雀境", "九品", "青铜级", "雀", 3},
	{2, "八品·市井雀手", "八品", "白银级", "雀", 3},
	{3, "六品·茶铺雀侠", "六品", "黄金级", "發", 4},
	{4, "四品·省城雀师", "四品", "铂金级", "發", 4},
	{5, "二品·岭南雀宗", "二品", "皇牌级", "中", 6},
	{6, "至尊·无双雀神", "至尊", "大满贯", "神", 0},
}

// 星级规则常量。
const (
	WinStars         = 1 // 常规胜场 +1 星
	LoseStars        = 1 // 输场 -1 星
	StreakBonusSmall = 1 // 3~5 连胜额外奖励
	StreakBonusBig   = 2 // ≥6 连胜额外奖励
	StreakSmallAt    = 3 // 小奖励起点连胜数
	StreakBigAt      = 6 // 大奖励起点连胜数
)

// Info 一个用户的段位展示信息。
type Info struct {
	TierIndex   int    `json:"tier_index"`
	TierName    string `json:"tier_name"`
	TierShort   string `json:"tier_short"`
	Grade       string `json:"grade"`
	Tile        string `json:"tile"`
	StarsInTier int    `json:"stars_in_tier"`
	StarsNeeded int    `json:"stars_needed"` // 0 表示至尊无上限
	TotalStars  int    `json:"total_stars"`
	Roman       string `json:"roman"` // 段内小段：初/I/II/III/IV/V
}

// InfoFromStars 由累计星数推导段位信息。
func InfoFromStars(total int) Info {
	remaining := total
	for i := 0; i < len(Tiers); i++ {
		t := Tiers[i]
		if t.StarsNeeded == 0 || remaining < t.StarsNeeded {
			return Info{
				TierIndex:   t.Index,
				TierName:    t.Name,
				TierShort:   t.Short,
				Grade:       t.Grade,
				Tile:        t.Tile,
				StarsInTier: remaining,
				StarsNeeded: t.StarsNeeded,
				TotalStars:  total,
				Roman:       roman(remaining),
			}
		}
		remaining -= t.StarsNeeded
	}
	// 不会到达：最后一段无上限
	t := Tiers[len(Tiers)-1]
	return Info{
		TierIndex: t.Index, TierName: t.Name, TierShort: t.Short, Grade: t.Grade,
		Tile: t.Tile, StarsNeeded: 0, TotalStars: total, Roman: roman(total),
	}
}

// roman 段内小段罗马数字，0 星显示「初」。
func roman(n int) string {
	if n <= 0 {
		return "初"
	}
	numerals := []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X"}
	if n > len(numerals) {
		return fmt.Sprintf("%d", n)
	}
	return numerals[n-1]
}

// Outcome 一名玩家在一场排位局结算中的变动。
type Outcome struct {
	Result      string // "win" / "draw" / "lose"
	StarsDelta  int    // 本场星级总变动（含连胜奖励，未含保底钳制）
	BonusStars  int    // 其中连胜奖励部分
	StreakAfter int    // 结算后连胜数（胜=原连胜+1，平/输=0）
}

// Settle 计算一场 4 人局的各玩家星级变动。
// scores: game_player_id -> 总得分；streaks: game_player_id -> 结算前连胜数。
// 返回 game_player_id -> Outcome。所有分数相同（如全场流局）按平局处理，星级不变。
func Settle(scores map[int64]int, streaks map[int64]int) map[int64]Outcome {
	out := make(map[int64]Outcome, len(scores))

	maxScore, minScore := maxMin(scores)
	for id, score := range scores {
		o := Outcome{Result: "draw", StreakAfter: 0}
		switch {
		case maxScore == minScore: // 全员同分
		case score == maxScore:
			o.Result = "win"
			o.StarsDelta = WinStars
			o.StreakAfter = streaks[id] + 1
			// 连胜奖励按本场获胜后的连胜数计
			if o.StreakAfter >= StreakBigAt {
				o.BonusStars = StreakBonusBig
			} else if o.StreakAfter >= StreakSmallAt {
				o.BonusStars = StreakBonusSmall
			}
			o.StarsDelta += o.BonusStars
		case score == minScore:
			o.Result = "lose"
			o.StarsDelta = -LoseStars
		}
		out[id] = o
	}
	return out
}

func maxMin(scores map[int64]int) (int, int) {
	maxScore, minScore := 0, 0
	first := true
	for _, s := range scores {
		if first {
			maxScore, minScore = s, s
			first = false
			continue
		}
		if s > maxScore {
			maxScore = s
		}
		if s < minScore {
			minScore = s
		}
	}
	return maxScore, minScore
}
