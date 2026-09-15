package handler

import (
	"testing"
	"time"

	"github.com/lk/zoek/backend/internal/model"
)

// 系统自动散台（5 小时超时）的时长口径：
// ended_at 必须按「最后一笔账」的时间计（真实收牌时刻），而不是超时扫描触发时刻。
// 记录页时长 = ended_at - started_at，若用扫描时刻会被 5 小时空窗拉长。

func TestAutoExpireEndedAtUsesLastEntry(t *testing.T) {
	r, _, s := testSetup(t)
	gameID, _ := createGame4P(t, r)

	// 造一笔已生效转分，并把开台/记账时间整体前移到 5 小时阈值之外
	players, err := s.GetActiveGamePlayers(gameID)
	if err != nil || len(players) < 2 {
		t.Fatalf("取玩家行失败: %v", err)
	}
	adj := addAdjustment(t, s, gameID, players[0].ID, players[1].ID, "confirmed", "adj-autoexp-1")

	startedAt := time.Now().Add(-7 * time.Hour)
	lastEntry := time.Now().Add(-6 * time.Hour)
	if err := s.DB.Model(&model.Game{}).Where("id = ?", gameID).
		Updates(map[string]interface{}{"started_at": startedAt, "created_at": startedAt}).Error; err != nil {
		t.Fatalf("回拨开台时间失败: %v", err)
	}
	if err := s.DB.Model(&model.ScoreAdjustment{}).Where("id = ?", adj.ID).
		Update("created_at", lastEntry).Error; err != nil {
		t.Fatalf("回拨记账时间失败: %v", err)
	}

	expired, cleaned, err := s.AutoExpireStaleGame(gameID)
	if err != nil {
		t.Fatalf("AutoExpireStaleGame 失败: %v", err)
	}
	if !expired || cleaned {
		t.Fatalf("expired=%v cleaned=%v, want true/false（有账的台应自动散台结算）", expired, cleaned)
	}

	game, err := s.GetGame(gameID)
	if err != nil {
		t.Fatalf("回查牌局失败: %v", err)
	}
	if game.Status != "ended" {
		t.Errorf("status = %q, want ended", game.Status)
	}
	if game.EndedAt == nil {
		t.Fatalf("ended_at 为空")
	}
	// ended_at 应等于最后一笔账时间（±1 分钟），而不是结算扫描时刻
	if diff := game.EndedAt.Sub(lastEntry); diff > time.Minute || diff < -time.Minute {
		t.Errorf("ended_at = %v, want 最后一笔账时间 %v（时长口径错会被 5h 空窗拉长）",
			game.EndedAt.Format(time.RFC3339), lastEntry.Format(time.RFC3339))
	}
	// 时长 = ended_at - started_at = 60 分钟，且落库到 duration_minutes 字段
	want := int(game.EndedAt.Sub(startedAt).Minutes())
	if want < 59 || want > 61 {
		t.Errorf("时长 = %d 分钟, want ≈60 分钟（最后一笔账 %v - 开台 %v）",
			want, lastEntry.Format("15:04"), startedAt.Format("15:04"))
	}
	if game.DurationMinutes < 59 || game.DurationMinutes > 61 {
		t.Errorf("落库 duration_minutes = %d, want ≈60（读侧直接取字段不再现算）", game.DurationMinutes)
	}
}
