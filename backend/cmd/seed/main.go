// cmd/seed/main.go — 生成雀友榜/个人数据用的模拟数据
// 用法: cd backend && go run ./cmd/seed
// 幂等: 以 openid=seed_* 识别模拟用户，重复执行会跳过已存在的用户和牌局。
//       进行中牌台以 invite_token_hash 里的 seed-live-* 标记做幂等。
//       已结束牌局不幂等，只想补进行中牌台时加 -games 0。
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

	// 与服务端保持一致的表结构（幂等）
	if err := st.AutoMigrate(); err != nil {
		fmt.Fprintf(os.Stderr, "数据库迁移失败: %v\n", err)
		os.Exit(1)
	}

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

	// ---- 3. 进行中的牌台 ----
	liveCreated := seedLiveTables(st, rng, mockUsers, hasReal, *realUserID, now)
	fmt.Printf("进行中牌台: 新生成 %d 桌（凑紧脚差一脚 / 齐人未记分 / 齐人已记分）\n", liveCreated)

	// ---- 4. 排位段位回填：重放全部已结束 4 人局 ----
	replayed := seedRanks(st)
	fmt.Printf("排位回填: 重放 %d 场 4 人局，段位与现有数据对齐\n", replayed)

	// 抽查零和：全部锁定提交总和应为 0
	var sum int64
	st.DB.Raw("SELECT COALESCE(SUM(score),0) FROM round_submissions rs " +
		"JOIN rounds r ON rs.round_id = r.id " +
		"JOIN games g ON r.game_id = g.id WHERE g.name LIKE '得闲开台 %'").Scan(&sum)
	fmt.Printf("锁定提交总和（应为 0）: %d\n", sum)
}

// seedLiveTables 生成三桌进行中的牌台，真实用户坐 1 号位做台主，方便真机直接查看：
//   - 凑紧脚差一脚：forming 3/4，4 号位空着（可试出示台码/长按空位换位/取消开台）
//   - 齐人未记分：active 4/4，首局开着但没人入分（可试取消开台/入分）
//   - 齐人已记分：active 4/4，已有若干入账局（可试结束散台→结算）
func seedLiveTables(st *store.Store, rng *rand.Rand, mockUsers []model.User, hasReal bool, realUserID int64, now time.Time) int {
	name := fmt.Sprintf("得闲开台 %d月%d日", now.Month(), now.Day())

	// withReal: 是否让真实用户坐 1 号位（牌台互斥规则下，真实用户只占一张台）
	tablePlayers := func(size int, withReal bool) []model.User {
		players := make([]model.User, 0, size)
		if withReal && hasReal {
			var real model.User
			if err := st.DB.First(&real, realUserID).Error; err == nil {
				players = append(players, real)
			}
		}
		pool := append([]model.User(nil), mockUsers...)
		rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		for _, u := range pool {
			if len(players) == size {
				break
			}
			players = append(players, u)
		}
		return players
	}

	createTable := func(marker string, size int, status string, startedAgo time.Duration, withReal bool) (*model.Game, []model.GamePlayer) {
		var existing model.Game
		if err := st.DB.Where("invite_token_hash = ?", marker).First(&existing).Error; err == nil {
			return nil, nil // 已生成过，跳过
		}
		players := tablePlayers(size, withReal)
		if len(players) < size {
			return nil, nil
		}
		createdAt := now.Add(-startedAgo - 30*time.Minute)
		game := model.Game{
			CreatorID:       players[0].ID,
			Name:            name,
			Status:          status,
			InviteTokenHash: marker,
			CreatedAt:       createdAt,
			UpdatedAt:       now,
			Version:         1,
		}
		if status != "forming" {
			startedAt := now.Add(-startedAgo)
			game.StartedAt = &startedAt
		}
		if err := st.DB.Create(&game).Error; err != nil {
			fmt.Printf("生成进行中牌台失败(%s): %v\n", marker, err)
			return nil, nil
		}
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
				Seat:             idx + 1,
				JoinedAt:         createdAt,
			}
			if err := st.DB.Create(&gp).Error; err != nil {
				return nil, nil
			}
			gps = append(gps, gp)
		}
		return &game, gps
	}

	created := 0

	// 1) 凑紧脚差一脚：forming 3/4，真实用户做台主（唯一占用的一张台）
	if _, gps := createTable("seed-live-forming-3", 3, "forming", 25*time.Minute, true); gps != nil {
		created++
	}

	// 2) 齐人未记分：active 4/4，首局开着没人入分（模拟用户的台）
	if g, _ := createTable("seed-live-active-fresh", 4, "active", 40*time.Minute, false); g != nil {
		round := model.Round{GameID: g.ID, RoundNumber: 1, Status: "open", CreatedAt: *g.StartedAt}
		if err := st.DB.Create(&round).Error; err != nil {
			fmt.Printf("生成首局失败(游戏 %d): %v\n", g.ID, err)
		}
		created++
	}

	// 3) 齐人已记分：active 4/4，若干入账局全部锁定（无进行中的局，可直接散台结算；模拟用户的台）
	if g, gps := createTable("seed-live-active-scored", 4, "active", 2*time.Hour, false); gps != nil {
		rounds := 3 + rng.Intn(3) // 3~5 局
		lockedAt := *g.StartedAt
		for rn := 1; rn <= rounds; rn++ {
			lockedAt = lockedAt.Add(time.Duration(8+rng.Intn(8)) * time.Minute)
			round := model.Round{GameID: g.ID, RoundNumber: rn, Status: "ready_for_next", LockedAt: &lockedAt, CreatedAt: lockedAt}
			if err := st.DB.Create(&round).Error; err != nil {
				break
			}
			scores := zeroSumScores(rng, len(gps))
			for pi, gp := range gps {
				sub := model.RoundSubmission{
					RoundID:      round.ID,
					GamePlayerID: gp.ID,
					Score:        scores[pi],
					RequestID:    fmt.Sprintf("seed-live-g%d-r%d-p%d", g.ID, rn, gp.ID),
					SubmittedAt:  lockedAt,
				}
				st.DB.Create(&sub)
			}
		}
		created++
	}

	return created
}

// seedRanks 清零全部用户排位数据后，按时间正序重放所有已结束 4 人局的排位结算，
// 使段位/连胜/赛季分与现有牌局数据一致（每次执行全量重算，天然幂等）。
func seedRanks(st *store.Store) int {
	if err := st.DB.Model(&model.User{}).Where("1 = 1").Updates(map[string]interface{}{
		"rank_stars": 0, "rank_wins": 0, "rank_draws": 0, "rank_losses": 0,
		"rank_streak": 0, "rank_best_streak": 0, "rank_points": 0,
	}).Error; err != nil {
		fmt.Fprintf(os.Stderr, "重置排位数据失败: %v\n", err)
		os.Exit(1)
	}
	if err := st.DB.Where("1 = 1").Delete(&model.RankSettlement{}).Error; err != nil {
		fmt.Fprintf(os.Stderr, "清空排位结算记录失败: %v\n", err)
		os.Exit(1)
	}

	var games []model.Game
	if err := st.DB.Where("status = 'ended'").Order("ended_at ASC, id ASC").Find(&games).Error; err != nil {
		fmt.Fprintf(os.Stderr, "读取已结束牌局失败: %v\n", err)
		os.Exit(1)
	}
	replayed := 0
	for _, g := range games {
		if err := st.SettleGameRank(g.ID); err == nil {
			var n int64
			st.DB.Model(&model.RankSettlement{}).Where("game_id = ?", g.ID).Count(&n)
			if n > 0 {
				replayed++
			}
		}
	}
	return replayed
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
