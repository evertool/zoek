package main

import (
	"fmt"

	"github.com/lk/zoek/backend/internal/rank"
)

func main() {
	for _, s := range []int{16, 50, 49} {
		i := rank.InfoFromStars(s)
		fmt.Printf("%2d star -> tier=%d %s (%s) inTier=%d peak=%v toPeak=%d\n",
			s, i.TierIndex, i.TierName, i.TierShort, i.StarsInTier, i.IsPeak, i.StarsToPeak)
	}
}
