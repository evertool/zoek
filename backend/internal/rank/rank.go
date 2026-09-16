// Package rank — 排位段位与星级结算（仅 4 人排位局适用）。
//
// 规则（PRD paiwei 设计稿）：
//   - 一场 4 人局结束，按总得分排名：最高分者胜 +1 星，最低分者输 -1 星，其余为平不扣星；
//   - 连胜奖励：胜场后连胜达到 3~5 场额外 +1 星，达到 6 场及以上额外 +2 星（平/输清零连胜）；
//   - 保底保护：星星最低为 0，雀仔段 0 星时输了不掉星；
//   - 段位：六段雀位，新手雀仔→街坊雀友→叹茶雀侠→老练雀师→岭南雀宗→无双雀神，星满自动升段，雀神无上限；
//   - 晋圣：无双雀神累计满 50 星后晋为「至尊·最强雀圣」（段内进阶，段位序号不变）。
package rank

import "fmt"

// Tier 描述一个段位。
type Tier struct {
	Index       int    // 1-6，1 最低
	Name        string // 全名，如 新手雀仔
	Short       string // 短名，如 雀仔
	Grade       string // 级别，如 青铜级
	Tile        string // 徽标用字：雀/友/侠/师/宗/神/圣
	StarsNeeded int    // 本段集满几星升下一段，0 表示无上限（雀神）
}

// Tiers 段位表，下标 0 为最低段。
var Tiers = []Tier{
	{1, "新手雀仔", "雀仔", "青铜级", "雀", 3},
	{2, "街坊雀友", "雀友", "白银级", "雀", 3},
	{3, "叹茶雀侠", "雀侠", "黄金级", "侠", 4},
	{4, "老练雀师", "雀师", "铂金级", "师", 4},
	{5, "岭南雀宗", "雀宗", "皇牌级", "宗", 6},
	{6, "无双雀神", "雀神", "大满贯", "神", 0},
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

// 雀神晋圣：无双雀神累计满 PeakStarsNeeded 星后晋为「至尊·最强雀圣」。
// 这是最高段内的进阶（段位序号仍为 6），不是第 7 个段位；晋圣后依然无星上限。
const (
	PeakStarsNeeded = 50
	PeakTierName    = "至尊·最强雀圣"
	PeakTierShort   = "雀圣"
	PeakTierTile    = "圣"
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
	Roman       string `json:"roman"`         // 段内小段：初/I/II/III/IV/V
	IsPeak      bool   `json:"is_peak"`       // 是否已晋为「至尊·最强雀圣」
	PeakStars   int    `json:"peak_stars"`    // 晋圣门槛（累计星数）；仅至尊段返回，其余为 0
	StarsToPeak int    `json:"stars_to_peak"` // 距晋圣还差几星；仅「至尊但未晋圣」时 > 0
}

// InfoFromStars 由累计星数推导段位信息。
func InfoFromStars(total int) Info {
	remaining := total
	for i := 0; i < len(Tiers); i++ {
		t := Tiers[i]
		if t.StarsNeeded == 0 || remaining < t.StarsNeeded {
			return peakAware(tierInfo(t, remaining, total))
		}
		remaining -= t.StarsNeeded
	}
	// 不会到达：最后一段无上限
	t := Tiers[len(Tiers)-1]
	return peakAware(tierInfo(t, total, total))
}

// tierInfo 组装某一段的展示信息；starsInTier 同时决定段内小段（罗马数字）。
func tierInfo(t Tier, starsInTier, total int) Info {
	return Info{
		TierIndex:   t.Index,
		TierName:    t.Name,
		TierShort:   t.Short,
		Grade:       t.Grade,
		Tile:        t.Tile,
		StarsInTier: starsInTier,
		StarsNeeded: t.StarsNeeded,
		TotalStars:  total,
		Roman:       roman(starsInTier),
	}
}

// peakAware 处理最高段（雀神）的晋圣进阶：累计星满 PeakStarsNeeded 后换成雀圣称号。
func peakAware(in Info) Info {
	if in.StarsNeeded != 0 { // 非最高段，无晋圣概念
		return in
	}
	in.PeakStars = PeakStarsNeeded
	if in.TotalStars >= PeakStarsNeeded {
		in.IsPeak = true
		in.TierName, in.TierShort, in.Tile = PeakTierName, PeakTierShort, PeakTierTile
	} else {
		in.StarsToPeak = PeakStarsNeeded - in.TotalStars
	}
	return in
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
