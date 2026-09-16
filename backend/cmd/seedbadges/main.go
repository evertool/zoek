// 一次性测试数据 seeder：为 user 1 造满徽章场次数据，26/27 补齐场次。
// 用完即删，不入库代码。
package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/lk/zoek/backend/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	userLKA   = int64(1) // lkahung：点亮全部徽章
	userWang  = int64(26)
	userInvoker = int64(27)
	totalGames  = 210
)

func main() {
	dsn := "root:Root@123@tcp(127.0.0.1:3306)/zoek?charset=utf8mb4&parseTime=true&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		panic(err)
	}

	// 取昵称快照
	type u struct {
		ID       int64
		Nickname string
	}
	var users []u
	if err := db.Table("users").Select("id, nickname").Find(&users).Error; err != nil {
		panic(err)
	}
	nick := map[int64]string{}
	for _, x := range users {
		nick[x.ID] = x.Nickname
	}

	rng := rand.New(rand.NewSource(42))
	names := []string{"周五围炉", "饮茶开局", "夜战雀馆", "周末手谈"}
	fillers := []int64{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}

	base := time.Date(2026, 9, 15, 18, 0, 0, 0, time.Local)
	fillerIdx := 0
	nonWinIdx := 0
	winsLKA := 0

	// user1 的 210 场剧本（按 ended_at 顺序）：
	// 1-3 输；4-9 连胜 6 场（连胜王）；10 大翻盘（9 局前 6 局负、终局 +30）；
	// 11-13 稳如泰山（终局 0 平）；14-21 手气王（期间峰值 ≥300）；其余 113 胜 + 76 输/平凑 120 胜
	for i := 0; i < totalGames; i++ {
		ended := base.Add(time.Duration(i*5) * time.Minute)
		dur := 60 + rng.Intn(91)
		started := ended.Add(-time.Duration(dur) * time.Minute)

		filler := fillers[fillerIdx%len(fillers)]
		fillerIdx++
		players := []int64{userLKA, userWang, userInvoker, filler}

		game := model.Game{
			CreatorID:           userLKA,
			Name:                names[i%len(names)],
			Status:              "ended",
			InviteTokenHash:     fmt.Sprintf("seed-badges-%d-%x", i, rng.Int63()),
			MembersLocked:       true,
			StartedAt:           &started,
			EndedAt:             &ended,
			SettlementUpdatedAt: &ended,
			DurationMinutes:     dur,
			Version:             1,
		}
		if err := db.Create(&game).Error; err != nil {
			panic(err)
		}

		gps := make([]model.GamePlayer, 0, 4)
		for seat, uid := range players {
			gps = append(gps, model.GamePlayer{
				GameID:           game.ID,
				UserID:           uid,
				NicknameSnapshot: nick[uid],
				Role:             "player",
				Seat:             seat + 1,
				Status:           "active",
			})
		}
		if err := db.Create(&gps).Error; err != nil {
			panic(err)
		}
		gpIDs := []int64{gps[0].ID, gps[1].ID, gps[2].ID, gps[3].ID}

		// 设计 user1 的局目标与逐局 chunks
		var nRounds int
		var chunks []int // user1 每局得分
		kind := "normal"
		switch {
		case i < 3: // 输
			kind = "lose"
			nRounds = 8
			chunks = splitTotal(rng, -80-rng.Intn(5)*20, nRounds, false)
		case i < 9: // 六连胜
			kind = "win"
			nRounds = 8
			chunks = splitTotal(rng, 200+rng.Intn(4)*20, nRounds, false)
		case i == 10: // 大翻盘：9 局，前 6 局累计 -240，后 3 局 +90 → 终局 +30
			kind = "comeback"
			nRounds = 9
			chunks = []int{-40, -40, -40, -40, -40, -40, 90, 90, 90}
		case i >= 11 && i <= 13: // 稳如泰山：终局 0
			kind = "steady"
			nRounds = 8
			chunks = splitTotal(rng, 0, nRounds, true)
		case i >= 14 && i <= 21: // 手气王：前 3 局 +150（峰值 450），后 5 局 -60 → 终局 +150
			kind = "lucky"
			nRounds = 8
			chunks = []int{150, 150, 150, -60, -60, -60, -60, -60}
		default:
			// 剩余 188 场（i=9 与 22..209）：need 总胜 120 → 已 7 胜（6 连胜 + 1 翻盘），i=9 也设为胜凑 8
			// 剩余 112 胜 + 76 平/负 ≈ 113 胜 76 负（含 steady 3 平不计负）
			if i == 9 || rng.Intn(100) < 60 {
				kind = "win"
				nRounds = 8
				chunks = splitTotal(rng, 200+rng.Intn(4)*20, nRounds, false)
			} else {
				kind = "lose"
				nRounds = 8
				chunks = splitTotal(rng, -(60 + rng.Intn(7)*20), nRounds, false)
			}
		}
		if kind == "win" || kind == "comeback" {
			winsLKA++
		}

		// 建 rounds + submissions（其余 3 人每局分摊 -chunk）
		var rounds []model.Round
		for r := 1; r <= nRounds; r++ {
			lockedAt := started.Add(time.Duration(r*10) * time.Minute)
			rounds = append(rounds, model.Round{GameID: game.ID, RoundNumber: r, Status: "locked", LockedAt: &lockedAt})
		}
		if err := db.Create(&rounds).Error; err != nil {
			panic(err)
		}
		var subs []model.RoundSubmission
		for r, ch := range chunks {
			others := splitNeg3(rng, -ch)
			vals := []int{ch, others[0], others[1], others[2]}
			for p := 0; p < 4; p++ {
				subs = append(subs, model.RoundSubmission{
					RoundID:      rounds[r].ID,
					GamePlayerID: gpIDs[p],
					Score:        vals[p],
					RequestID:    fmt.Sprintf("seed-%d-%d-%d", game.ID, r, p),
				})
			}
		}
		if err := db.CreateInBatches(subs, 100).Error; err != nil {
			panic(err)
		}

		_ = nonWinIdx
	}
	fmt.Printf("seeded %d games, user1 wins=%d\n", totalGames, winsLKA)
}

// splitTotal 把 target 拆成 n 个整数，和恰为 target；final0=true 时保证和为 0
func splitTotal(rng *rand.Rand, target, n int, final0 bool) []int {
	if final0 {
		// 交替 ±k，和为 0
		out := make([]int, n)
		sum := 0
		for j := 0; j < n-1; j++ {
			v := (5 + rng.Intn(8)) * 5
			if j%2 == 1 {
				v = -v
			}
			out[j] = v
			sum += v
		}
		out[n-1] = -sum
		return out
	}
	// 随机游走拆分：先随机 n-1 个，最后一个补差（控制幅度）
	out := make([]int, n)
	sum := 0
	sign := 1
	if target < 0 {
		sign = -1
	}
	for j := 0; j < n-1; j++ {
		v := sign * (rng.Intn(60) + 5)
		out[j] = v
		sum += v
	}
	out[n-1] = target - sum
	return out
}

// splitNeg3 把 v 拆成 3 个整数（和为 v）
func splitNeg3(rng *rand.Rand, v int) []int {
	a := rng.Intn(41) - 20
	b := rng.Intn(41) - 20
	return []int{a, b, v - a - b}
}
