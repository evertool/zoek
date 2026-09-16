// 一次性：按真实牌局数据回填 users.rank_*（仅 1/26/27），保持 rank_stars 不动。用完即删。
package main

import (
	"fmt"

	"github.com/lk/zoek/backend/internal/model"
	"github.com/lk/zoek/backend/internal/store"
	"gorm.io/gorm"
)

func main() {
	s, err := store.NewFromConfig("mysql",
		"root:Root@123@tcp(127.0.0.1:3306)/zoek?charset=utf8mb4&parseTime=true&loc=Local",
		"silent", nil)
	if err != nil {
		panic(err)
	}
	db := s.DB // Store.DB 是公开字段
	for _, uid := range []int64{1, 26, 27} {
		var games []model.Game
		db.Where("status='ended' AND ended_at IS NOT NULL AND id IN (SELECT game_id FROM game_players WHERE user_id=?)", uid).
			Order("ended_at ASC").Find(&games)
		wins, draws, losses, points, cur, best := 0, 0, 0, 0, 0, 0
		for _, g := range games {
			totals, _, err := s.AggregateSettlement(g.ID)
			if err != nil {
				continue
			}
			var mine *store.PlayerTotal
			for i := range totals {
				if totals[i].UserID == uid {
					mine = &totals[i]
				}
			}
			if mine == nil {
				continue
			}
			points += int(mine.TotalScore)
			if mine.Rank == 1 && mine.TotalScore > 0 {
				wins++
				cur++
				if cur > best {
					best = cur
				}
			} else {
				cur = 0
				if mine.TotalScore == 0 {
					draws++
				} else {
					losses++
				}
			}
		}
		if err := db.Model(&model.User{}).Where("id=?", uid).Updates(map[string]interface{}{
			"rank_wins": wins, "rank_draws": draws, "rank_losses": losses,
			"rank_points": points, "rank_streak": cur, "rank_best_streak": best,
		}).Error; err != nil {
			panic(err)
		}
		fmt.Printf("user %d: %d胜 %d平 %d负 净胜%+d 连胜%d 最佳%d\n", uid, wins, draws, losses, points, cur, best)
	}
	_ = gorm.Expr
}
