// examples/stream/main.go
//
// 流式处理示例：逐根推 K 线，链式 API 每次全量分析。

package main

import (
	"fmt"
	"log"
	"math"
	"time"

	chanlun "github.com/bambuo/chan"
)

func main() {
	total := 200
	initSize := 50
	all := trendData(total)

	// 首次全量分析
	prev := chanlun.NewAnalysis(all[:initSize], chanlun.NewBiConfig()).
		MergeKlines().DetectFractals().BuildBis().BuildSegments().
		DetectPivots().ClassifyTrends().DetectDeviations().
		DetectSignals().ScoreSignals().Result()

	fmt.Printf("初始化完成（%d 根），笔=%d 线段=%d 中枢=%d\n\n",
		initSize, len(prev.Bis), len(prev.Segments), len(prev.Pivots))

	signalCount := 0

	// 逐根推送
	for i := initSize; i < total; i++ {
		// 每次把已有数据 + 新 K 线传进去
		klines := append(all[:i], all[i])
		result := chanlun.NewAnalysis(klines, chanlun.NewBiConfig()).
			MergeKlines().DetectFractals().BuildBis().BuildSegments().
			DetectPivots().ClassifyTrends().DetectDeviations().
			DetectSignals().ScoreSignals().Result()

		if len(result.Signals) > signalCount {
			for _, sig := range result.Signals[signalCount:] {
				signalCount++
				log.Printf("[信号#%d] %s @%d 价格=%.2f 强度=%.2f",
					signalCount, name(sig.Type), sig.Index, sig.Price, sig.Strength)
			}
		}
	}

	fmt.Printf("\n流式完成。共 %d 个信号。\n", signalCount)
}

func name(t chanlun.SignalType) string {
	switch t {
	case chanlun.BuyPoint1:
		return "一买"
	case chanlun.BuyPoint2:
		return "二买"
	case chanlun.BuyPoint3:
		return "三买"
	case chanlun.SellPoint1:
		return "一卖"
	case chanlun.SellPoint2:
		return "二卖"
	case chanlun.SellPoint3:
		return "三卖"
	default:
		return fmt.Sprintf("未知(%d)", t)
	}
}

func trendData(n int) []chanlun.Kline {
	k := make([]chanlun.Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range k {
		p := float64(i) * 0.3
		var tr float64
		if i < n/2 {
			tr = float64(i) * 0.3
		} else {
			tr = float64(n-i) * 0.3
		}
		mid := 100.0 + tr + 10.0*math.Sin(p)
		k[i] = chanlun.Kline{
			Time:       chanlun.DateTime{Time: t.Add(time.Duration(i) * time.Hour)},
			Open:       mid - 0.5,
			High:       mid + 2.0,
			Low:        mid - 1.5,
			Close:      mid + 0.3,
			BaseVolume: 1000,
		}
	}
	return k
}
