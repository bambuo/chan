// cmd/compare/main.go
//
// 对比测试工具：运行 Go 缠论分析并导出各阶段结果到 JSON。
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	chanlun "github.com/bambuo/chan"
)

func main() {
	n := 200
	klines := sampleData(n)

	cfg := alignedConfig()

	result := chanlun.NewAnalysis(klines, cfg).
		MergeKlines().
		DetectFractals().
		BuildBis().
		BuildSegments().
		DetectPivots().
		ClassifyTrends().
		DetectDeviations().
		DetectSignals().
		DetectSegSignals().
		Result()

	// 输出摘要
	printSummary(result)

	// 导出 JSON
	out, err := json.MarshalIndent(output{
		MergedKlines: mergeKlineSummary(result.MergedKlines),
		Fractals:     result.Fractals,
		Bis:          result.Bis,
		MergedBis:    mergedBiSummary(result.MergedBis),
		Segments:     result.Segments,
		Pivots:       result.Pivots,
		Trends:       result.Trends,
		Deviations:   result.Deviations,
		Signals:      result.Signals,
	}, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("go_result.json", out, 0o644); err != nil {
		panic(err)
	}
	fmt.Println("\n结果已写入 go_result.json")
}

func printSummary(r *chanlun.Result) {
	fmt.Printf("=== Go 分析结果 ===\n")
	fmt.Printf("合并K线=%d 分型=%d 笔=%d 合笔=%d 线段=%d 中枢=%d 走势=%d 背驰=%d 信号=%d\n",
		len(r.MergedKlines), len(r.Fractals), len(r.Bis), len(r.MergedBis),
		len(r.Segments), len(r.Pivots), len(r.Trends), len(r.Deviations), len(r.Signals))

	fmt.Println("\n--- 线段 ---")
	for i, s := range r.Segments {
		startBi, endBi := -1, -1
		if len(s.BiList) > 0 {
			startBi = s.BiList[0].Bi.StartIndex
			endBi = s.BiList[len(s.BiList)-1].Bi.EndIndex
		}
		fmt.Printf("  seg_%d: dir=%v sure=%v start=%d end=%d bi[%d:%d]\n",
			i, s.Direction, s.IsSure, s.StartIndex, s.EndIndex, startBi, endBi)
	}

	fmt.Println("\n--- 中枢 ---")
	for i, z := range r.Pivots {
		fmt.Printf("  zs_%d: ZG=%.2f ZD=%.2f GG=%.2f DD=%.2f state=%v bi[%d:%d]\n",
			i, z.ZG, z.ZD, z.PeakHigh, z.PeakLow, z.State, z.BeginBiIdx, z.EndBiIdx)
	}

	fmt.Println("\n--- 信号 ---")
	for _, sig := range r.Signals {
		fmt.Printf("  sig: type=%v sub=%v idx=%d price=%.2f strength=%.2f\n",
			sig.Type, sig.SubType, sig.Index, sig.Price, sig.Strength)
	}
	if len(r.SegSignals) > 0 {
		fmt.Println("\n--- 线段级信号 ---")
		for _, sig := range r.SegSignals {
			fmt.Printf("  seg_sig: type=%v sub=%v idx=%d price=%.2f strength=%.2f\n",
				sig.Type, sig.SubType, sig.Index, sig.Price, sig.Strength)
		}
	}
	fmt.Println()
}

// alignedConfig 返回与 Python 参考实现对齐的配置。
func alignedConfig() chanlun.Config {
	cfg := chanlun.DefaultConfig()
	// 与 Python main.py 配置对齐
	cfg.BiStrict = true
	cfg.BiAlgo = "normal"
	cfg.BiFxCheck = "strict"
	cfg.GapAsKl = true
	cfg.BiEndIsPeak = true
	cfg.BiAllowSubPeak = true

	cfg.SegAlgo = "chan"
	cfg.LeftSegMethod = "peak"

	cfg.ZsAlgo = "normal"
	cfg.ZsCombine = true
	cfg.ZsCombineMode = "zs"
	cfg.OneBiZs = false

	cfg.BspDivergenceRate = math.Inf(1)
	cfg.BspMinZsCnt = 1
	cfg.Bsp1OnlyMultiBiZs = true
	cfg.BspMaxBs2Rate = 0.9999
	cfg.BspMacdAlgo = "peak"
	cfg.Bsp1Peak = true
	cfg.BspType = "1,1p,2,2s,3a,3b,support,resist,breakUp,breakDn"
	cfg.Bsp2Follow1 = true
	cfg.Bsp3Follow1 = true
	cfg.Bsp3Peak = false
	cfg.StrictBsp3 = false

	cfg.EnableSegBSP = true

	return cfg
}

type output struct {
	MergedKlines []mergeKlineItem    `json:"merged_klines"`
	Fractals     []chanlun.Fractal   `json:"fractals"`
	Bis          []chanlun.Bi        `json:"bis"`
	MergedBis    []mergedBiItem      `json:"merged_bis"`
	Segments     []chanlun.Segment   `json:"segments"`
	Pivots       []chanlun.Pivot     `json:"pivots"`
	Trends       []chanlun.Trend     `json:"trends"`
	Deviations   []chanlun.Deviation `json:"deviations"`
	Signals      []chanlun.Signal    `json:"signals"`
	SegSignals   []chanlun.Signal    `json:"seg_signals,omitempty"`
}

type mergeKlineItem struct {
	Index int     `json:"index"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Dir   string  `json:"dir,omitempty"`
}

type mergedBiItem struct {
	StartIndex    int     `json:"start_index"`
	EndIndex      int     `json:"end_index"`
	StartPrice    float64 `json:"start_price"`
	EndPrice      float64 `json:"end_price"`
	Direction     string  `json:"direction"`
	OriginalCount int     `json:"original_count"`
}

func mergeKlineSummary(kls []chanlun.Kline) []mergeKlineItem {
	r := make([]mergeKlineItem, len(kls))
	for i, k := range kls {
		r[i] = mergeKlineItem{
			Index: i,
			High:  k.High,
			Low:   k.Low,
		}
	}
	return r
}

func mergedBiSummary(bis []chanlun.MergedBi) []mergedBiItem {
	r := make([]mergedBiItem, len(bis))
	for i, b := range bis {
		dirStr := "up"
		if b.Bi.Direction == chanlun.DirDown {
			dirStr = "down"
		}
		r[i] = mergedBiItem{
			StartIndex:    b.Bi.StartIndex,
			EndIndex:      b.Bi.EndIndex,
			StartPrice:    b.Bi.StartPrice,
			EndPrice:      b.Bi.EndPrice,
			Direction:     dirStr,
			OriginalCount: b.OriginalCount,
		}
	}
	return r
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
