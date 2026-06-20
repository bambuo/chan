// examples/stream/main.go
//
// 实时流式处理示例。
// 模拟 WebSocket 逐根推送 K 线，使用 StreamEngine 增量分析。
// 展示：O(1) 增量更新、并发安全、增量信号回调。

package main

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	chanlun "github.com/bambuo/chan"
)

func main() {
	// 创建增量引擎（使用严笔标准）
	stream, err := chanlun.NewStreamEngine(chanlun.NewBiConfig())
	if err != nil {
		log.Fatalf("引擎初始化失败: %v", err)
	}

	// 模拟数据源：200 根 K 线
	totalKlines := 200
	initSize := 50
	allKlines := generateTrendData(totalKlines)

	// 全量初始化
	_, err = stream.Init(allKlines[:initSize])
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	fmt.Printf("初始化完成（%d 根 K 线），开始增量推送...\n\n", initSize)

	var mu sync.Mutex
	signalCount := 0

	// 逐根增量推送
	for i := initSize; i < totalKlines; i++ {
		inc := stream.AddKline(allKlines[i])

		if len(inc.NewSignals) > 0 {
			mu.Lock()
			for _, sig := range inc.NewSignals {
				signalCount++
				snap := inc.Snapshot
				log.Printf("[信号#%d] %s | 价格=%.2f | 强度=%.2f | 笔=%d 线段=%d 中枢=%d",
					signalCount, signalName(sig.Type), sig.Price, sig.Strength,
					len(snap.Bis), len(snap.Segments), len(snap.Pivots))
			}
			mu.Unlock()
		}
	}

	snap := stream.Snapshot()
	fmt.Printf("\n流式处理完成。共 %d 个信号。\n", signalCount)
	fmt.Printf("最终: 已合并K线=%d 笔=%d 线段=%d 中枢=%d\n",
		len(snap.MergedKlines), len(snap.Bis), len(snap.Segments), len(snap.Pivots))
}

func signalName(t chanlun.SignalType) string {
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

func generateTrendData(n int) []chanlun.Kline {
	klines := make([]chanlun.Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		phase := float64(i) * 0.3
		osc := 10.0 * math.Sin(phase)
		var trend float64
		if i < n/2 {
			trend = float64(i) * 0.3
		} else {
			trend = float64(n-i) * 0.3
		}
		mid := 100.0 + trend + osc
		klines[i] = chanlun.Kline{
			Time:       chanlun.DateTime{Time: t.Add(time.Duration(i) * time.Hour)},
			Open:       mid - 0.5,
			High:       mid + 2.0,
			Low:        mid - 1.5,
			Close:      mid + 0.3,
			BaseVolume: 1000,
		}
	}
	return klines
}
