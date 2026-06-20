// examples/basic/main.go
//
// 缠论算法引擎基本使用示例。
// 演示：创建引擎、处理 K 线、读取结果。

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	csv "github.com/gocarina/gocsv"

	chanlun "github.com/bambuo/chan"
)

func main() {
	// 1. 创建引擎（默认配置 = 旧笔标准）
	engine, err := chanlun.NewEngine(chanlun.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// 2. 准备 K 线数据（通常来自交易所 API）
	klines := readKline("/Users/johana/Codes/github/canlun/dataset/ETHUSDT_1m.csv")

	// 3. 运行完整分析
	result, err := engine.Process(klines)
	if err != nil {
		panic(err)
	}

	// 4. 读取中间结果
	fmt.Printf("K线包含处理: %d → %d (%.1f%% 合并)\n",
		len(klines), len(result.MergedKlines),
		float64(len(klines)-len(result.MergedKlines))/float64(len(klines))*100)
	fmt.Printf("分型: %d\n", len(result.Fractals))
	fmt.Printf("笔: %d\n", len(result.Bis))
	fmt.Printf("线段: %d\n", len(result.Segments))
	fmt.Printf("中枢: %d\n", len(result.Pivots))
	fmt.Printf("走势类型: %d\n", len(result.Trends))
	fmt.Printf("背驰信号: %d\n", len(result.Deviations))
	fmt.Printf("买卖点信号: %d\n", len(result.Signals))

	// 5. 打印走势类型
	for i, trend := range result.Trends {
		name := "盘整"
		if trend.Type == chanlun.TrendUp {
			name = "📈 上涨趋势"
		} else if trend.Type == chanlun.TrendDown {
			name = "📉 下跌趋势"
		}
		fmt.Printf("\n走势 %d: %s | 中枢数=%d | 索引=[%d,%d] | 完成=%v\n",
			i, name, len(trend.Pivots), trend.StartIndex, trend.EndIndex, trend.IsComplete)
	}

	// 6. 打印买卖点信号
	for i, sig := range result.Signals {
		typeName := ""
		switch sig.Type {
		case chanlun.BuyPoint1:
			typeName = "一买 🟢"
		case chanlun.BuyPoint2:
			typeName = "二买 🟢"
		case chanlun.BuyPoint3:
			typeName = "三买 🟢"
		case chanlun.SellPoint1:
			typeName = "一卖 🔴"
		case chanlun.SellPoint2:
			typeName = "二卖 🔴"
		case chanlun.SellPoint3:
			typeName = "三卖 🔴"
		}
		fmt.Printf("  信号 %d: %s | 级别=%s | 索引=%d | 价格=%.2f | 强度=%.2f\n",
			i, typeName, sig.Level, sig.Index, sig.Price, sig.Strength)
	}

	// 7. JSON 序列化示例
	jsonBytes, _ := json.MarshalIndent(result.Signals, "", "  ")
	fmt.Printf("\n信号 JSON:\n%s\n", jsonBytes)
}

// generateSampleData 生成示例 K 线数据（正弦波振荡 + 趋势）。
func generateSampleData(n int) []chanlun.Kline {
	klines := make([]chanlun.Kline, n)
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		phase := float64(i) * 0.4
		osc := 8.0 * math.Sin(phase)
		trend := float64(i) * 0.15
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

func readKline(fp string) []chanlun.Kline {
	if fp == "" {
		return generateSampleData(200)
	}

	header := []byte("time,open,high,low,close,baseVolume\n")
	data, err := os.ReadFile(fp)
	if err != nil {
		panic(err)
	}
	data = append(header, data...)

	var klines []chanlun.Kline

	err = csv.UnmarshalBytes(data, &klines)
	if err != nil {
		panic(err)
	}
	return klines
}
