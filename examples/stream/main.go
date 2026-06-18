// examples/stream/main.go
//
// 实时流式处理示例。
// 模拟 WebSocket 逐根推送 K 线，调用 Update() 增量分析。
// 展示：滑动窗口增量更新、并发安全、信号回调。

package main

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/bambuo/chan"
)

func main() {
	// 创建引擎（允许并发调用）
	engine, err := chanlun.NewEngine(chanlun.NewBiConfig()) // 使用严笔标准
	if err != nil {
		log.Fatalf("引擎初始化失败: %v", err)
	}

	// 模拟数据源：200 根 K 线分批推送
	totalCandles := 200
	batchSize := 50
	allCandles := generateTrendData(totalCandles)

	fmt.Printf("开始流式处理 %d 根 K 线...\n\n", totalCandles)
	var wg sync.WaitGroup

	// 分批模拟推送
	for batchStart := 0; batchStart < totalCandles; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > totalCandles {
			batchEnd = totalCandles
		}
		batch := allCandles[batchStart:batchEnd]

		wg.Add(1)
		go func(candles []chanlun.Candle) {
			defer wg.Done()
			for _, c := range candles {
				// 模拟每个新 K 线到达
				result, err := engine.Update(c)
				if err != nil {
					log.Printf("Update 错误: %v", err)
					continue
				}

				// 仅在有新信号时打印
				if len(result.Signals) > 0 {
					lastSig := result.Signals[len(result.Signals)-1]
					typeName := signalName(lastSig.Type)
					log.Printf("[信号] %s | 价格=%.2f | 强度=%.2f | 分型=%d 笔=%d 线段=%d 中枢=%d",
						typeName, lastSig.Price, lastSig.Strength,
						len(result.Fractals), len(result.Bis),
						len(result.Segments), len(result.Pivots))
				}
			}
		}(batch)
	}

	wg.Wait()
	fmt.Println("\n流式处理完成。")
}

func signalName(t chanlun.SignalType) string {
	switch t {
	case chanlun.BuyPoint1:
		return "一买 🟢"
	case chanlun.BuyPoint2:
		return "二买 🟢"
	case chanlun.BuyPoint3:
		return "三买 🟢"
	case chanlun.SellPoint1:
		return "一卖 🔴"
	case chanlun.SellPoint2:
		return "二卖 🔴"
	case chanlun.SellPoint3:
		return "三卖 🔴"
	default:
		return fmt.Sprintf("未知(%d)", t)
	}
}

// generateTrendData 生成先涨后跌的走势数据。
func generateTrendData(n int) []chanlun.Candle {
	candles := make([]chanlun.Candle, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range candles {
		phase := float64(i) * 0.3
		osc := 10.0 * math.Sin(phase)
		var trend float64
		if i < n/2 {
			trend = float64(i) * 0.3 // 前半段上涨
		} else {
			trend = float64(n-i) * 0.3 // 后半段下跌
		}
		mid := 100.0 + trend + osc
		candles[i] = chanlun.Candle{
			Time:   t.Add(time.Duration(i) * time.Hour),
			Open:   mid - 0.5,
			High:   mid + 2.0,
			Low:    mid - 1.5,
			Close:  mid + 0.3,
			Volume: 1000,
		}
	}
	return candles
}
