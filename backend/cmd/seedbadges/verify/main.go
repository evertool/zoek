// 一次性验证：调真实 GetUserBadges / InfoFromStars 检查 1/26/27。用完即删。
package main

import (
	"fmt"

	"github.com/lk/zoek/backend/internal/rank"
	"github.com/lk/zoek/backend/internal/store"
)

func main() {
	s, err := store.NewFromConfig("mysql",
		"root:Root@123@tcp(127.0.0.1:3306)/zoek?charset=utf8mb4&parseTime=true&loc=Local",
		"silent", nil)
	if err != nil {
		panic(err)
	}
	for _, uid := range []int64{1, 26, 27} {
		b, err := s.GetUserBadges(uid)
		if err != nil {
			panic(err)
		}
		fmt.Printf("== user %d ==\n", uid)
		for _, x := range b.Badges {
			mark := "·"
			if x.Unlocked {
				mark = "✔"
			}
			fmt.Printf("  %s %-8s %d/%d\n", mark, x.Name, x.Current, x.Target)
		}
	}
	for _, stars := range []int{16, 50, 49} {
		i := rank.InfoFromStars(stars)
		fmt.Printf("%2d★ -> %s(%s) inTier=%d peak=%v toPeak=%d\n",
			stars, i.TierName, i.TierShort, i.StarsInTier, i.IsPeak, i.StarsToPeak)
	}
}
