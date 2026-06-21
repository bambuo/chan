// examples/basic/main.go
//
// 缠论算法引擎使用示例 — 链式 API。

package main

import (
	"fmt"
	"math"
	"time"

	chanlun "github.com/bambuo/chan"
)

func main() {
	klines := sampleData(200)

	result := chanlun.NewAnalysis(klines, chanlun.DefaultConfig()).
		MergeKlines().
		DetectFractals().
		BuildBis().
		BuildSegments().
		DetectPivots().
		ClassifyTrends().
		DetectDeviations().
		DetectSignals().
		ScoreSignals().
		Result()

	fmt.Printf("分析结果：\n")
	fmt.Printf("  笔=%d  线段=%d  中枢=%d  背驰=%d  信号=%d\n",
		len(result.Bis), len(result.Segments),
		len(result.Pivots), len(result.Deviations),
		len(result.Signals))

	// 走马灯走势类型
	for i, t := range result.Trends {
		label := "盘整"
		if t.Type == chanlun.TrendUp {
			label = "📈 上涨趋势"
		} else if t.Type == chanlun.TrendDown {
			label = "📉 下跌趋势"
		}
		fmt.Printf("走势 %d: %s | 中枢=%d | [%d,%d] | 完成=%v\n",
			i, label, len(t.Pivots), t.StartIndex, t.EndIndex, t.IsComplete)
	}

	// 买卖点
	for _, sig := range result.Signals {
		name := ""
		switch sig.Type {
		case chanlun.BuyPoint1:
			name = "一买 🟢"
		case chanlun.BuyPoint2:
			name = "二买 🟢"
		case chanlun.BuyPoint3:
			name = "三买 🟢"
		case chanlun.SellPoint1:
			name = "一卖 🔴"
		case chanlun.SellPoint2:
			name = "二卖 🔴"
		case chanlun.SellPoint3:
			name = "三卖 🔴"
		}
		fmt.Printf("  %s @%d 价格=%.2f 强度=%.2f\n",
			name, sig.Index, sig.Price, sig.Strength)
	}
}

func sampleData(n int) []chanlun.Kline {
	k := make([]chanlun.Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range k {
		p := float64(i) * 0.4
		tr := float64(i) * 0.15
		mid := 100.0 + tr + 8.0*math.Sin(p)
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
