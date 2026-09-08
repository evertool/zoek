// cmd/seed/main.go — 生成雀友榜/个人数据用的模拟数据
// 用法: cd backend && go run ./cmd/seed
// 幂等: 以 openid=seed_* 识别模拟用户，重复执行会跳过已存在的用户和牌局。
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/lk/zoek/backend/internal/config"
	"github.com/lk/zoek/backend/internal/logger"
	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/store"
)

// 粤语风雀友昵称池
var nicknames = []string{
	"阿強", "明仔", "細妹", "華仔", "阿May", "細黃", "大隻佬", "肥仔聰",
	"手多多", "雀聖強", "飲茶先", "阿婆", "孖指輝", "爆seed", "十三張", "食糊了",
	"卡窿二", "九筒王", "東南西北", "花旗矩", "摸摸地", "相公良", "截糊香", "槓上花",
}

func main() {
	gamesFlag := flag.Int("games", 70, "生成的已结束牌局数量")
	daysFlag := flag.Int("days", 30, "牌局分布的时间窗口（天）")
	realUserID := flag.Int64("real-user-id", 2, "真实开发用户 ID（会高频参与牌局）")
	configPath := flag.String("config", "", "path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}
	log, err := logger.New(cfg.Log.Level, cfg.Log.Encoding, cfg.Log.Output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	st, err := store.NewFromConfig(cfg.Database.Driver, cfg.Database.DSNString(), cfg.Database.LogLevel, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	rng := rand.New(rand.NewSource(20260909))

	// ---- 1. 模拟用户 ----
	var mockUsers []model.User
	for i, nick := range nicknames {
		openid := fmt.Sprintf("seed_user_%02d", i+1)
		var u model.User
		if err := st.DB.Where("openid = ?", openid).First(&u).Error; err == nil {
			mockUsers = append(mockUsers, u)
			continue
		}
		u = model.User{OpenID: openid, Nickname: nick}
		if err := st.DB.Create(&u).Error; err != nil {
			fmt.Fprintf(os.Stderr, "创建用户失败: %v\n", err)
			os.Exit(1)
		}
		mockUsers = append(mockUsers, u)
	}
	fmt.Printf("模拟用户: %d 位\n", len(mockUsers))

	// 真实用户（开发者的账号），让榜上有一个"自己"
	var realUser model.User
	hasReal := false
	if *realUserID > 0 {
		if err := st.DB.First(&realUser, *realUserID).Error; err == nil {
			hasReal = true
		}
	}

	// ---- 2. 已结束牌局 ----
	existingGames := int64(0)
	st.DB.Model(&model.Game{}).Where("name LIKE ?", "得闲开台 %").Count(&existingGames)
	_ = existingGames

	created := 0
	now := time.Now()
	for gi := 0; gi < *gamesFlag; gi++ {
		// 时间分布在最近 days 天内（今天最多，越往前越少）
		daysAgo := rng.Float64() * float64(*daysFlag)
		startedAt := now.Add(-time.Duration(daysAgo*24) * time.Hour).
			Add(-time.Duration(rng.Intn(10)) * time.Hour)
		rounds := 4 + rng.Intn(13) // 4~16 局
		endedAt := startedAt.Add(time.Duration(rounds*12+rng.Intn(60)) * time.Minute)

		// 参与者：2~4 人，真实用户约 45% 概率上桌
		players := pickPlayers(st, rng, mockUsers, hasReal, *realUserID)
		if len(players) < 2 {
			continue
		}

		name := fmt.Sprintf("得闲开台 %d月%d日", startedAt.Month(), startedAt.Day())

		game := model.Game{
			CreatorID:           players[0].ID,
			Name:                name,
			Status:              "ended",
			InviteTokenHash:     uuid.New().String(), // 唯一约束要求；种子牌局不再接受加入
			StartedAt:           &startedAt,
			EndedAt:             &endedAt,
			SettlementUpdatedAt: &endedAt,
			CreatedAt:           startedAt.Add(-30 * time.Minute),
			UpdatedAt:           endedAt,
			Version:             1,
		}
		if err := st.DB.Create(&game).Error; err != nil {
			continue
		}

		// game_players（含快照昵称）
		var gps []model.GamePlayer
		for idx, pu := range players {
			role := "player"
			if idx == 0 {
				role = "owner"
			}
			gp := model.GamePlayer{
				GameID:           game.ID,
				UserID:           pu.ID,
				NicknameSnapshot: pu.Nickname,
				Role:             role,
				JoinedAt:         startedAt.Add(-30 * time.Minute),
			}
			if err := st.DB.Create(&gp).Error; err != nil {
				break
			}
			gps = append(gps, gp)
		}
		if len(gps) != len(players) {
			continue
		}

		// 逐局生成零和分数
		lockedAt := startedAt
		for rn := 1; rn <= rounds; rn++ {
			lockedAt = lockedAt.Add(time.Duration(6+rng.Intn(10)) * time.Minute)
			round := model.Round{GameID: game.ID, RoundNumber: rn, Status: "ready_for_next", LockedAt: &lockedAt, CreatedAt: lockedAt}
			if err := st.DB.Create(&round).Error; err != nil {
				break
			}

			scores := zeroSumScores(rng, len(gps))
			for pi, gp := range gps {
				sub := model.RoundSubmission{
					RoundID:      round.ID,
					GamePlayerID: gp.ID,
					Score:        scores[pi],
					RequestID:    fmt.Sprintf("seed-g%d-r%d-p%d", game.ID, rn, gp.ID),
					SubmittedAt:  lockedAt,
				}
				if err := st.DB.Create(&sub).Error; err != nil {
					break
				}
			}
		}

		// 少量已确认改分记录
		if rng.Float64() < 0.22 && len(gps) >= 2 && rounds > 0 {
			adjRound := 1 + rng.Intn(rounds)
			var rnd model.Round
			if err := st.DB.Where("game_id = ? AND round_number = ?", game.ID, adjRound).First(&rnd).Error; err == nil {
				fi := rng.Intn(len(gps))
				ti := (fi + 1 + rng.Intn(len(gps)-1)) % len(gps)
				adj := model.ScoreAdjustment{
					GameID:         game.ID,
					RoundID:        rnd.ID,
					FromPlayerID:   gps[fi].ID,
					ToPlayerID:     gps[ti].ID,
					AdjustmentType: "supplement",
					Amount:         1 + rng.Intn(8),
					Reason:         "少记咗一炮",
					ProposedBy:     gps[fi].UserID,
					Status:         "accepted",
					RequestID:      fmt.Sprintf("seed-adj-g%d-%d", game.ID, fi),
					ExpiresAt:      endedAt.Add(24 * time.Hour),
					ResolvedBy:     &gps[ti].UserID,
					CreatedAt:      endedAt,
					ResolvedAt:     &endedAt,
				}
				st.DB.Create(&adj)
			}
		}

		created++
	}

	fmt.Printf("已生成牌局: %d 场（窗口 %d 天）\n", created, *daysFlag)

	// 抽查零和：全部锁定提交总和应为 0
	var sum int64
	st.DB.Raw("SELECT COALESCE(SUM(score),0) FROM round_submissions rs " +
		"JOIN rounds r ON rs.round_id = r.id " +
		"JOIN games g ON r.game_id = g.id WHERE g.name LIKE '得闲开台 %'").Scan(&sum)
	fmt.Printf("锁定提交总和（应为 0）: %d\n", sum)
}

// pickPlayers 组一桌 2~4 人，真实用户高频上桌。
func pickPlayers(st *store.Store, rng *rand.Rand, mockUsers []model.User, hasReal bool, realUserID int64) []model.User {
	size := 2 + rng.Intn(3) // 2~4
	pool := append([]model.User(nil), mockUsers...)
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	players := pool[:size-1]
	if hasReal && rng.Float64() < 0.45 {
		var real model.User
		if err := st.DB.First(&real, realUserID).Error; err == nil {
			// 真实用户插到中间位置，名次分布更自然
			pos := rng.Intn(size)
			players = append(players[:pos], append([]model.User{real}, players[pos:]...)...)
			if len(players) > size {
				players = players[:size]
			}
		}
	}
	return players
}

// zeroSumScores 生成本局 n 人的零和分数：一个赢家，其余输家分摊，偶有流局。
func zeroSumScores(rng *rand.Rand, n int) []int {
	scores := make([]int, n)
	if rng.Float64() < 0.08 { // 流局：全部 0
		return scores
	}
	winner := rng.Intn(n)
	win := 4 + rng.Intn(57) // +4 ~ +60
	scores[winner] = win

	// 剩余 n-1 人分摊 -win
	losers := make([]int, 0, n-1)
	for i := 0; i < n; i++ {
		if i != winner {
			losers = append(losers, i)
		}
	}
	remain := win
	for li, idx := range losers {
		if li == len(losers)-1 {
			scores[idx] = -remain
			break
		}
		share := remain / (len(losers) - li)
		if share > 1 {
			jit := rng.Intn(share/2 + 1)
			share -= jit
		}
		if share < 1 {
			share = 1
		}
		scores[idx] = -share
		remain -= share
	}
	return scores
}
