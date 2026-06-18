package chanlun

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// candleJSON 匹配文档中的 JSON 格式
type candleJSON struct {
	OpenTime  int64   `json:"open_time"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	CloseTime int64   `json:"close_time"`
}

type btcData struct {
	Symbol    string       `json:"symbol"`
	Timeframe string       `json:"timeframe"`
	Candles   []candleJSON `json:"candles"`
}

// loadBTCData 从 JSON 文件加载 K 线数据。
func loadBTCData(path string) ([]Candle, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var data btcData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	candles := make([]Candle, len(data.Candles))
	for i, c := range data.Candles {
		candles[i] = Candle{
			Time:   time.UnixMilli(c.OpenTime),
			Open:   c.Open,
			High:   c.High,
			Low:    c.Low,
			Close:  c.Close,
			Volume: c.Volume,
		}
	}

	return candles, nil
}

// 实盘数据路径
const btcDataPath = "../../docs/BTCUSDT_1h.json"
const ethDataPath = "../../docs/ETHUSDT_1h.json"
const solDataPath = "../../docs/SOLUSDT_1h.json"
const bnbDataPath = "../../docs/BNBUSDT_1h.json"

func TestBTCRealData_FullPipeline(t *testing.T) {
	candles, err := loadBTCData(btcDataPath)
	if err != nil {
		t.Fatalf("load BTC data: %v", err)
	}
	t.Logf("Loaded %d BTCUSDT 1h candles", len(candles))
	t.Logf("Date range: %s ~ %s", candles[0].Time.Format("2006-01-02 15:04"), candles[len(candles)-1].Time.Format("2006-01-02 15:04"))
	t.Logf("Price range: %.2f ~ %.2f", minPrice(candles), maxPrice(candles))

	engine, _ := NewEngine(DefaultConfig())
	start := time.Now()
	result, err := engine.Process(candles)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Engine.Process failed: %v", err)
	}

	t.Logf("├─ Processing time: %v", elapsed)
	t.Logf("├─ MergedCandles:  %d (%.1f%% merged)", 
		len(result.MergedCandles), 
		float64(len(candles)-len(result.MergedCandles))/float64(len(candles))*100)
	t.Logf("├─ Fractals:       %d", len(result.Fractals))
	t.Logf("├─ Bis:            %d", len(result.Bis))
	t.Logf("├─ MergedBis:      %d (%.1f%% merged)", 
		len(result.MergedBis), 
		float64(len(result.Bis)-len(result.MergedBis))/float64(len(result.Bis))*100)
	t.Logf("├─ Segments:       %d", len(result.Segments))
	t.Logf("├─ Pivots:         %d", len(result.Pivots))
	t.Logf("├─ Trends:         %d", len(result.Trends))
	t.Logf("├─ Deviations:     %d", len(result.Deviations))
	t.Logf("└─ Signals:        %d", len(result.Signals))

	// 详细输出
	if len(result.Trends) > 0 {
		t.Logf("\n=== 走势类型分析 ===")
		for i, tr := range result.Trends {
			trendName := "盘整"
			if tr.Type == TrendUp {
				trendName = "上涨趋势"
			} else if tr.Type == TrendDown {
				trendName = "下跌趋势"
			}
			t.Logf("  Trend %d: %s | %d pivots | candles [%d, %d] | complete=%v",
				i, trendName, len(tr.Pivots), tr.StartIndex, tr.EndIndex, tr.IsComplete)
			if tr.CompleteReason != "" {
				t.Logf("    → 完成原因: %s", tr.CompleteReason)
			}
		}
	}

	if len(result.Pivots) > 0 {
		t.Logf("\n=== 中枢分析 ===")
		for i, p := range result.Pivots {
			stateName := "形成"
			switch p.State {
			case PivotForming:
				stateName = "形成中"
			case PivotFormed:
				stateName = "已形成"
			case PivotExtending:
				stateName = "延伸中"
			case PivotExpanded:
				stateName = "扩展升级"
			case PivotEnlarged:
				stateName = "扩张升级"
			case PivotDestroyed:
				stateName = "被破坏"
			}
			t.Logf("  Pivot %d: %s | ZG=%.2f ZD=%.2f | %d segs | level=%d",
				i, stateName, p.ZG, p.ZD, len(p.Segments), p.Level)
		}
	}

	if len(result.Signals) > 0 {
		t.Logf("\n=== 买卖点信号 ===")
		for i, s := range result.Signals {
			sigName := "一买"
			switch s.Type {
			case BuyPoint2:
				sigName = "二买"
			case BuyPoint3:
				sigName = "三买"
			case SellPoint1:
				sigName = "一卖"
			case SellPoint2:
				sigName = "二卖"
			case SellPoint3:
				sigName = "三卖"
			}
			t.Logf("  Signal %d: %s | level=%s | index=%d | price=%.2f | strength=%.2f",
				i, sigName, s.Level, s.Index, s.Price, s.Strength)
		}
	}
}

func TestBTCRealData_DetailedSegments(t *testing.T) {
	candles, err := loadBTCData(btcDataPath)
	if err != nil {
		t.Fatalf("load BTC data: %v", err)
	}

	// 只处理最近 1000 根 K 线进行详细分析
	window := candles[len(candles)-1000:]
	t.Logf("Analyzing last %d candles (%s ~ %s)",
		len(window), window[0].Time.Format("2006-01-02"), window[len(window)-1].Time.Format("2006-01-02"))

	engine, _ := NewEngine(DefaultConfig())
	result, err := engine.Process(window)
	if err != nil {
		t.Fatalf("Engine.Process failed: %v", err)
	}

	t.Logf("\n=== 最近 %d 根 K 线分析 ===", len(window))
	t.Logf("Fractals:  %d", len(result.Fractals))
	t.Logf("Bis:       %d", len(result.Bis))
	t.Logf("Segments:  %d", len(result.Segments))
	t.Logf("Pivots:    %d", len(result.Pivots))
	t.Logf("Trends:    %d", len(result.Trends))
	t.Logf("Deviations: %d", len(result.Deviations))
	t.Logf("Signals:   %d", len(result.Signals))

	// 输出线段详情
	for i, seg := range result.Segments {
		dir := "↑"
		if seg.Direction == DirDown {
			dir = "↓"
		}
		t.Logf("  Segment %d: %s | range=[%d,%d] | price=[%.2f, %.2f] | %d bis | broken=%v",
			i, dir, seg.StartIndex, seg.EndIndex, seg.Bottom, seg.Top, len(seg.BiList), seg.IsBroken)
	}
}

func TestETHRealData(t *testing.T) {
	candles, err := loadBTCData(ethDataPath) // same JSON format
	if err != nil {
		t.Fatalf("load ETH data: %v", err)
	}
	t.Logf("Loaded %d ETHUSDT 1h candles", len(candles))
	t.Logf("Price range: %.2f ~ %.2f", minPrice(candles), maxPrice(candles))

	engine, _ := NewEngine(DefaultConfig())
	result, err := engine.Process(candles)
	if err != nil {
		t.Fatalf("Engine.Process failed: %v", err)
	}

	t.Logf("Fractals: %d, Bis: %d, Segments: %d, Pivots: %d, Trends: %d, Signals: %d",
		len(result.Fractals), len(result.Bis), len(result.Segments),
		len(result.Pivots), len(result.Trends), len(result.Signals))
}

func TestBTC_MultipleWindows(t *testing.T) {
	// 不同数据量窗口的算法性能对比
	candles, err := loadBTCData(btcDataPath)
	if err != nil {
		t.Fatalf("load BTC data: %v", err)
	}

	windows := []int{100, 500, 2000, 5000, 10000}
	engine, _ := NewEngine(DefaultConfig())

	t.Logf("\n=== 不同数据量窗口性能对比 ===")
	t.Logf("%10s | %10s | %8s | %8s | %8s | %8s | %8s | %8s",
		"窗口大小", "耗时", "分型", "笔", "线段", "中枢", "背驰", "信号")
	t.Logf("%s", "────────────┼────────────┼──────────┼──────────┼──────────┼──────────┼──────────┼──────────")

	for _, w := range windows {
		if w > len(candles) {
			w = len(candles)
		}
		window := candles[len(candles)-w:]

		start := time.Now()
		result, err := engine.Process(window)
		elapsed := time.Since(start)

		if err != nil {
			t.Logf("%10d | %10v | ERROR: %v", w, elapsed, err)
			continue
		}

		t.Logf("%10d | %10v | %8d | %8d | %8d | %8d | %8d | %8d",
			w, elapsed, len(result.Fractals), len(result.Bis),
			len(result.Segments), len(result.Pivots),
			len(result.Deviations), len(result.Signals))
	}
}

func TestBTC_RecentSegmentDetail(t *testing.T) {
	// 最近 200 根 K 线的逐线段细节
	candles, err := loadBTCData(btcDataPath)
	if err != nil {
		t.Fatalf("load BTC data: %v", err)
	}

	window := candles[len(candles)-200:]
	engine, _ := NewEngine(DefaultConfig())
	result, err := engine.Process(window)
	if err != nil {
		t.Fatalf("Engine.Process failed: %v", err)
	}

	t.Logf("\n=== 最近 %d 根 K 线逐线段分析 ===", len(window))
	for i, seg := range result.Segments {
		dir := "↑"
		if seg.Direction == DirDown {
			dir = "↓"
		}

		// 计算线段实际价格变化
		change := (seg.Top - seg.Bottom) / seg.Bottom * 100

		t.Logf("  Segment %d: dir=%s range=[%d,%d] price=[%.2f, %.2f] Δ=%.2f%% bis=%d broken=%v",
			i, dir, seg.StartIndex, seg.EndIndex, seg.Bottom, seg.Top, change, len(seg.BiList), seg.IsBroken)

		// 输出构成线段的笔
		for j, b := range seg.BiList {
			biDir := "↑"
			if b.Direction == DirDown {
				biDir = "↓"
			}
			t.Logf("    Bi %d: dir=%s range=[%d,%d] price=[%.2f, %.2f]",
				j, biDir, b.StartIndex, b.EndIndex, b.StartPrice, b.EndPrice)
		}
	}

	if len(result.Deviations) > 0 {
		t.Logf("\n=== 背驰信号 ===")
		for i, d := range result.Deviations {
			devDir := "顶背驰"
			if d.Direction == DirDown {
				devDir = "底背驰"
			}
			t.Logf("  Deviation %d: %s | type=%s | level=%d | force_before=%.4f force_after=%.4f",
				i, devDir, d.Type, d.Level, d.ForceBefore, d.ForceAfter)
		}
	}
}

// 辅助函数
func minPrice(candles []Candle) float64 {
	m := candles[0].Low
	for _, c := range candles {
		if c.Low < m {
			m = c.Low
		}
	}
	return m
}

func maxPrice(candles []Candle) float64 {
	m := candles[0].High
	for _, c := range candles {
		if c.High > m {
			m = c.High
		}
	}
	return m
}
