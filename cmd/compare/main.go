// cmd/compare/main.go
//
// 缠论多级别对比测试工具。
// 支持从 CSV 读取 1m K 线数据，自动聚合为多周期后执行 BuildMultiLevelChain，
// 导出带周期切换的可视化 HTML 页面。
//
// 用法：
//
//	go run .                                                        // 合成数据
//	go run . --csv=../../testdata/BTCUSDT_1m.csv --rows=5000         // 真实行情
//	go run . --csv=../../testdata/BTCUSDT_1m.csv --rows=5000 --html  // 生成 HTML
package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	chanlun "github.com/bambuo/chan"
	"github.com/bambuo/chan/multi"
	"github.com/bambuo/chan/types"
)

var aggregationFactors = []struct {
	Interval string
	Factor   int // 聚合倍数（相对于前一级别）
}{
	{"1m", 1},
	{"5m", 5},
	{"15m", 3},
	{"1h", 4},
}

func main() {
	csvPath := flag.String("csv", "", "行情 CSV 文件路径")
	rowCount := flag.Int("rows", 200, "读取的 K 线行数")
	genHTML := flag.Bool("html", false, "生成可视化 HTML 页面")
	flag.Parse()

	// 读取原始 K 线（视为 1m 级别）
	var baseKlines []chanlun.Kline
	if *csvPath != "" {
		var err error
		baseKlines, err = readCSV(*csvPath, *rowCount)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 CSV 失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("从 %s 读取 %d 条 K 线\n", *csvPath, len(baseKlines))
	} else {
		baseKlines = sampleData(*rowCount)
		fmt.Printf("使用 %d 条合成 K 线\n", len(baseKlines))
	}

	cfg := alignedConfig()

	// 构建多级别输入：从 1m 依次聚合
	levels := buildLevels(baseKlines)
	if len(levels) < 1 {
		fmt.Fprintln(os.Stderr, "K 线数据不足以构建任何级别")
		os.Exit(1)
	}

	// 执行多级别链分析
	result := multi.BuildMultiLevelChain(levels, cfg)
	if result == nil {
		fmt.Fprintln(os.Stderr, "多级别分析失败")
		os.Exit(1)
	}

	// 打印各级别摘要
	for i, lr := range result.Levels {
		r := lr.Result
		if r == nil {
			continue
		}
		fmt.Printf("Level[%d] %s: 合并K线=%d 笔=%d 线段=%d 中枢=%d 信号=%d\n",
			i, lr.Interval, len(r.MergedKlines),
			len(r.Bis), len(r.Segments), len(r.Pivots), len(r.Signals))
	}
	fmt.Printf("共振=%d", result.Resonance)
	if result.Nesting != nil {
		fmt.Printf(" 区间套精度=%.2f%%", result.Nesting.Accuracy*100)
	}
	fmt.Println()

	// 输出多级别 JSON
	jsonBytes := marshalMultiResult(result)
	if err := os.WriteFile("go_multi_result.json", jsonBytes, 0o644); err != nil {
		panic(err)
	}
	fmt.Println("\n多级别结果已写入 go_multi_result.json")

	if *genHTML {
		if err := renderHTML("chan_result.html", result); err != nil {
			fmt.Fprintf(os.Stderr, "生成 HTML 失败: %v\n", err)
		}
	}
}

// buildLevels 从 1m K 线依次聚合构建多级别输入。
func buildLevels(baseKlines []types.Kline) []multi.LevelInput {
	if len(baseKlines) == 0 {
		return nil
	}
	levels := make([]multi.LevelInput, 0, len(aggregationFactors))
	levels = append(levels, multi.LevelInput{
		Interval: aggregationFactors[0].Interval,
		Klines:   baseKlines,
	})
	current := baseKlines
	for i := 1; i < len(aggregationFactors); i++ {
		factor := aggregationFactors[i].Factor
		aggregated := aggregateKlines(current, factor)
		if len(aggregated) < 10 {
			break
		}
		levels = append(levels, multi.LevelInput{
			Interval: aggregationFactors[i].Interval,
			Klines:   aggregated,
		})
		current = aggregated
	}
	return levels
}

// aggregateKlines 将 K 线按 factor 倍聚合。
// factor=5 时，每 5 根聚合成 1 根：取首根 Open、末根 Close、区间最高 High、最低 Low、成交量加和。
func aggregateKlines(klines []types.Kline, factor int) []types.Kline {
	if factor <= 1 || len(klines) == 0 {
		return klines
	}
	n := len(klines) / factor
	result := make([]types.Kline, 0, n)
	for i := 0; i < n*factor; i += factor {
		end := i + factor
		if end > len(klines) {
			end = len(klines)
		}
		group := klines[i:end]
		c := types.Kline{
			Time:  group[0].Time,
			Open:  group[0].Open,
			Close: group[end-i-1].Close,
			High:  group[0].High,
			Low:   group[0].Low,
		}
		for _, k := range group[1:] {
			if k.High > c.High {
				c.High = k.High
			}
			if k.Low < c.Low {
				c.Low = k.Low
			}
			c.BaseVolume += k.BaseVolume
		}
		result = append(result, c)
	}
	return result
}

// marshalMultiResult 将 MultiLevelResult 序列化为前端友好的 JSON。
func marshalMultiResult(mr *multi.MultiLevelResult) []byte {
	type levelJSON struct {
		Interval    string            `json:"interval"`
		MergedCount int               `json:"mergedCount"`
		BiCount     int               `json:"biCount"`
		SegCount    int               `json:"segCount"`
		PivotCount  int               `json:"pivotCount"`
		SignalCount int               `json:"signalCount"`
		Merged      []mergeKlineItem  `json:"merged,omitempty"`
		Bis         []mergedBiItem    `json:"bis,omitempty"`
		Segments    []sanitizedSeg    `json:"segments,omitempty"`
		Pivots      []sanitizedPivot  `json:"pivots,omitempty"`
		Signals     []sanitizedSignal `json:"signals,omitempty"`
		Deviations  []sanitizedDev    `json:"deviations,omitempty"`
	}
	type multiJSON struct {
		Levels    []levelJSON                  `json:"levels"`
		Resonance int                          `json:"resonance"`
		Nesting   *types.IntervalNestingResult `json:"nesting,omitempty"`
	}

	out := multiJSON{Resonance: mr.Resonance, Nesting: mr.Nesting}
	for _, lr := range mr.Levels {
		r := lr.Result
		if r == nil {
			continue
		}
		lj := levelJSON{
			Interval:    lr.Interval,
			MergedCount: len(r.MergedKlines),
			BiCount:     len(r.Bis),
			SegCount:    len(r.Segments),
			PivotCount:  len(r.Pivots),
			SignalCount: len(r.Signals),
			Merged:      mergeKlineSummary(r.MergedKlines),
			Bis:         mergedBiSummary(r.MergedBis),
			Segments:    sanitizeSegments(r.Segments),
			Pivots:      sanitizePivots(r.Pivots),
			Signals:     sanitizeSignals(r.Signals),
			Deviations:  sanitizeDeviations(r.Deviations),
		}
		out.Levels = append(out.Levels, lj)
	}

	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	return raw
}

func sanitizeFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// ── 带 Inf/NaN 清理的序列化类型 ──

type sanitizedSeg struct {
	StartIndex   int            `json:"start_index"`
	EndIndex     int            `json:"end_index"`
	Direction    string         `json:"direction"`
	BiList       []mergedBiItem `json:"biList,omitempty"`
	Top          float64        `json:"top"`
	Bottom       float64        `json:"bottom"`
	IsBroken     bool           `json:"isBroken"`
	BreakType    string         `json:"breakType,omitempty"`
	ConfirmIndex int            `json:"confirmIndex"`
	IsSure       bool           `json:"isSure"`
}

type sanitizedPivot struct {
	StartIndex   int     `json:"start_index"`
	EndIndex     int     `json:"end_index"`
	ZG           float64 `json:"zg"`
	ZD           float64 `json:"zd"`
	GG           float64 `json:"gg"`
	DD           float64 `json:"dd"`
	PeakHigh     float64 `json:"peakHigh,omitempty"`
	PeakLow      float64 `json:"peakLow,omitempty"`
	OverlapCount int     `json:"overlapCount"`
	Level        int     `json:"level"`
	SourceLevel  string  `json:"sourceLevel,omitempty"`
	State        string  `json:"state"`
	Direction    string  `json:"direction"`
	BeginBiIdx   int     `json:"beginBiIdx,omitempty"`
	EndBiIdx     int     `json:"endBiIdx,omitempty"`
	IsSure       bool    `json:"isSure,omitempty"`
}

type sanitizedSignal struct {
	Type     int     `json:"type"`
	SubType  string  `json:"subType,omitempty"`
	Level    string  `json:"level"`
	Index    int     `json:"index"`
	Price    float64 `json:"price"`
	Strength float64 `json:"strength"`
}

type sanitizedDev struct {
	Type        string  `json:"type"`
	Direction   int     `json:"direction"`
	PriceHigh   float64 `json:"priceHigh"`
	ForceBefore float64 `json:"forceBefore"`
	ForceAfter  float64 `json:"forceAfter"`
}

func sanitizeSegments(segs []types.Segment) []sanitizedSeg {
	out := make([]sanitizedSeg, len(segs))
	for i, s := range segs {
		dirStr := "up"
		if s.Direction == chanlun.DirDown {
			dirStr = "down"
		}
		btStr := ""
		if s.BreakType == chanlun.BreakStd {
			btStr = "std"
		} else if s.BreakType == chanlun.BreakStroke {
			btStr = "stroke"
		}
		ss := sanitizedSeg{
			StartIndex: s.StartIndex, EndIndex: s.EndIndex,
			Direction: dirStr, IsSure: s.IsSure,
			Top: sanitizeFloat(s.Top), Bottom: sanitizeFloat(s.Bottom),
			IsBroken: s.IsBroken, BreakType: btStr, ConfirmIndex: s.ConfirmIndex,
		}
		if len(s.BiList) > 0 {
			ss.BiList = mergedBiSummary(s.BiList)
		}
		out[i] = ss
	}
	return out
}

func sanitizePivots(pivots []types.Pivot) []sanitizedPivot {
	out := make([]sanitizedPivot, len(pivots))
	for i, p := range pivots {
		dirStr := "up"
		if p.Direction == chanlun.DirDown {
			dirStr = "down"
		}
		stateStr := ""
		switch p.State {
		case chanlun.PivotFormed:
			stateStr = "formed"
		case chanlun.PivotExtending:
			stateStr = "extending"
		case chanlun.PivotDestroyed:
			stateStr = "destroyed"
		default:
			stateStr = "forming"
		}
		out[i] = sanitizedPivot{
			StartIndex: p.StartIndex, EndIndex: p.EndIndex,
			ZG: sanitizeFloat(p.ZG), ZD: sanitizeFloat(p.ZD),
			GG: sanitizeFloat(p.GG), DD: sanitizeFloat(p.DD),
			PeakHigh: sanitizeFloat(p.PeakHigh), PeakLow: sanitizeFloat(p.PeakLow),
			OverlapCount: p.OverlapCount, Level: p.Level,
			SourceLevel: p.SourceLevel, State: stateStr,
			Direction: dirStr, BeginBiIdx: p.BeginBiIdx, EndBiIdx: p.EndBiIdx,
			IsSure: p.IsSure,
		}
	}
	return out
}

func sanitizeSignals(sigs []types.Signal) []sanitizedSignal {
	out := make([]sanitizedSignal, len(sigs))
	for i, s := range sigs {
		out[i] = sanitizedSignal{
			Type: int(s.Type), SubType: string(s.SubType),
			Level: s.Level, Index: s.Index,
			Price: sanitizeFloat(s.Price), Strength: sanitizeFloat(s.Strength),
		}
	}
	return out
}

func sanitizeDeviations(devs []types.Deviation) []sanitizedDev {
	out := make([]sanitizedDev, len(devs))
	for i, d := range devs {
		out[i] = sanitizedDev{
			Type: d.Type, Direction: int(d.Direction),
			PriceHigh:   sanitizeFloat(d.PriceHigh),
			ForceBefore: sanitizeFloat(d.ForceBefore),
			ForceAfter:  sanitizeFloat(d.ForceAfter),
		}
	}
	return out
}

// ── CSV 读取 ──

func readCSV(path string, maxRows int) ([]chanlun.Kline, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件: %w", err)
	}
	defer f.Close()

	br := bufio.NewReaderSize(f, 256*1024)
	r := csv.NewReader(br)
	r.ReuseRecord = true
	r.FieldsPerRecord = -1

	var klines []chanlun.Kline
	for len(klines) < maxRows {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取行 %d: %w", len(klines)+1, err)
		}
		if len(rec) < 6 {
			continue
		}
		tm, err := parseTime(rec[0])
		if err != nil {
			continue
		}
		open, _ := strconv.ParseFloat(rec[1], 64)
		high, _ := strconv.ParseFloat(rec[2], 64)
		low, _ := strconv.ParseFloat(rec[3], 64)
		closeP, _ := strconv.ParseFloat(rec[4], 64)
		vol, _ := strconv.ParseFloat(rec[5], 64)
		if high < low {
			high, low = low, high
		}
		klines = append(klines, chanlun.Kline{
			Time:       chanlun.DateTime{Time: tm},
			Open:       open,
			High:       high,
			Low:        low,
			Close:      closeP,
			BaseVolume: vol,
		})
	}
	return klines, nil
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		time.RFC3339,
	} {
		t, err := time.Parse(layout, strings.TrimSpace(s))
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间: %s", s)
}

// ── 配置 ──

func alignedConfig() chanlun.Config {
	cfg := chanlun.DefaultConfig()
	cfg.BiFxCheck = "loss"
	cfg.BiEndIsPeak = false
	cfg.BiAllowSubPeak = true
	cfg.SegAlgo = "def"
	cfg.LeftSegMethod = "all"
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

// ── 输出结构体 ──

type mergeKlineItem struct {
	Index int     `json:"index"`
	Time  string  `json:"time"`
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

type mergedBiItem struct {
	StartIndex int     `json:"start_index"`
	EndIndex   int     `json:"end_index"`
	StartPrice float64 `json:"start_price"`
	EndPrice   float64 `json:"end_price"`
	Direction  string  `json:"direction"`
}

func mergeKlineSummary(kls []types.Kline) []mergeKlineItem {
	r := make([]mergeKlineItem, len(kls))
	for i, k := range kls {
		ts := ""
		if !k.Time.IsZero() {
			ts = k.Time.Format(time.RFC3339)
		}
		r[i] = mergeKlineItem{
			Index: i, Time: ts,
			Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		}
	}
	return r
}

func mergedBiSummary(bis []types.MergedBi) []mergedBiItem {
	r := make([]mergedBiItem, len(bis))
	for i, b := range bis {
		dirStr := "up"
		if b.Bi.Direction == chanlun.DirDown {
			dirStr = "down"
		}
		r[i] = mergedBiItem{
			StartIndex: b.Bi.StartIndex, EndIndex: b.Bi.EndIndex,
			StartPrice: b.Bi.StartPrice, EndPrice: b.Bi.EndPrice,
			Direction: dirStr,
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
			Time:       chanlun.DateTime{Time: t.Add(time.Duration(i) * time.Minute)},
			Open:       mid - 0.5,
			High:       mid + 2.0,
			Low:        mid - 1.5,
			Close:      mid + 0.3,
			BaseVolume: 1000,
		}
	}
	return k
}

// ── HTML 渲染 ──

const maxRenderCandles = 10000

func renderHTML(path string, mr *multi.MultiLevelResult) error {
	return os.WriteFile(path, []byte(htmlTemplate), 0o644)
}

var htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate">
<meta http-equiv="Pragma" content="no-cache">
<meta http-equiv="Expires" content="0">
<title>缠论多级别分析</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#1a1d23;color:#dcdfe6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;height:100vh;overflow:hidden}
#app{height:100vh;display:flex;flex-direction:column}
.ctrl{background:#21252b;border-bottom:1px solid #333842;padding:6px 14px;display:flex;align-items:center;gap:10px;flex-wrap:wrap;min-height:38px;z-index:5;font-size:13px}
.ctrl .label{color:#878d96;white-space:nowrap}
.ctrl .stat{color:#dcdfe6;font-size:12px}
.ctrl .stat b{color:#f5a623}
.ctrl .sep{width:1px;height:20px;background:#333842;flex-shrink:0}
.tog{display:inline-flex;align-items:center;gap:4px;padding:3px 9px;border:1px solid #444a58;border-radius:4px;background:transparent;color:#878d96;cursor:pointer;font-size:12px;transition:all .12s;user-select:none;white-space:nowrap}
.tog:hover{border-color:#5c8aff;color:#dcdfe6}
.tog.on{background:#5c8aff22;border-color:#5c8aff;color:#5c8aff}
.tog .dot{width:7px;height:7px;border-radius:50%;display:inline-block;flex-shrink:0}
.tog.on .dot{box-shadow:0 0 4px currentColor}
.tog.active-period{background:#5c8aff33;border-color:#5c8aff;color:#5c8aff;font-weight:600}
.chart-wrap{flex:1;position:relative;min-height:300px;overflow:hidden}
.legend{position:absolute;top:6px;left:10px;z-index:5;background:rgba(33,37,43,0.85);border:1px solid #333842;border-radius:4px;padding:5px 8px;font-size:11px;color:#878d96;pointer-events:none;display:flex;flex-direction:column;gap:2px}
.legend-row{display:flex;align-items:center;gap:5px}
.ld{width:8px;height:3px;border-radius:1px;flex-shrink:0}
.ld.bi-u{background:#5c8aff}
.ld.bi-d{background:#f5a623}
.ld.seg{background:#ab47bc;width:10px;height:2.5px}
.ld.pvt{background:rgba(255,255,255,0.18);border:1px solid rgba(255,255,255,0.3);width:10px;height:10px}
.ld.bsp-b{background:#00e676;width:6px;height:6px;border-radius:50%}
.ld.bsp-s{background:#ff1744;width:6px;height:6px;border-radius:50%}
.status{background:#21252b;border-top:1px solid #333842;padding:2px 14px;font-size:12px;color:#878d96;min-height:24px;display:flex;align-items:center;gap:12px}
.err{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;background:rgba(26,29,35,0.9);z-index:20;gap:12px}
.err .msg{color:#ef5350;font-size:15px}
.err .hint{color:#878d96;font-size:13px;max-width:480px;text-align:center;line-height:1.6}
</style>
</head>
<body>
<div id="app">
<div class="ctrl">
<span class="label">BTCUSDT</span>
<div class="sep"></div>
<span class="label" style="font-size:12px">周期:</span>
<div id="periodBtns"></div>
<div class="sep"></div>
<span class="stat" id="stats">加载中...</span>
<div class="sep"></div>
<button class="tog on" data-layer="bi"><span class="dot" style="background:#5c8aff"></span>笔</button>
<button class="tog on" data-layer="seg"><span class="dot" style="background:#ab47bc"></span>线段</button>
<button class="tog on" data-layer="pvt"><span class="dot" style="background:rgba(255,255,255,0.4)"></span>中枢</button>
<button class="tog on" data-layer="sig"><span class="dot" style="background:#00e676"></span>信号</button>
</div>
<div class="chart-wrap" id="chartWrap">
<div class="legend">
<div class="legend-row"><div class="ld bi-u"></div><span>笔(上)</span></div>
<div class="legend-row"><div class="ld bi-d"></div><span>笔(下)</span></div>
<div class="legend-row"><div class="ld seg"></div><span>线段</span></div>
<div class="legend-row"><div class="ld pvt"></div><span>中枢</span></div>
<div class="legend-row"><div class="ld bsp-b"></div><span>买入</span></div>
<div class="legend-row"><div class="ld bsp-s"></div><span>卖出</span></div>
</div>
<div id="errorBox" class="err" style="display:none">
<div class="msg" id="errMsg"></div>
<div class="hint">请通过HTTP服务器运行此页面<br/>终端执行: <code style="background:#333;padding:2px 6px;border-radius:3px;">python3 -m http.server 8080</code></div>
</div>
</div>
<div class="status" id="statusBar"></div>
</div>

<script type="importmap">
{"imports":{"lightweight-charts":"https://unpkg.com/lightweight-charts@5.0.3/dist/lightweight-charts.standalone.production.mjs"}}
</script>
<script type="module">
import{createChart,CandlestickSeries}from'lightweight-charts';

const MAX_CANDLES = 10000;

async function loadData(){
	try{
		var resp = await fetch('go_multi_result.json');
		if(!resp.ok) throw new Error('HTTP '+resp.status);
		return await resp.json();
	}catch(e){
		showError('加载数据失败: '+e.message);
		throw e;
	}
}

function showError(msg){
	document.getElementById('errorBox').style.display='flex';
	document.getElementById('errMsg').textContent=msg;
}

function toSec(ts){return typeof ts==='string'?Math.floor(new Date(ts).getTime()/1000):Math.floor(ts/1000);}

var chart, series, ctx, wrap;
var allLevels = [];
var currentLevelIdx = 0;
var showLayers = {bi:true, seg:true, pvt:true, sig:true};
var klCache = [];

function start(data){
	allLevels = data.levels || [];
	if(allLevels.length===0){showError('无级别数据');return;}

	document.getElementById('stats').innerHTML='级别 <b>'+allLevels.length+'</b> 共振 <b>'+(data.resonance||0)+'</b>';

	// 周期按钮
	var periodBtns = document.getElementById('periodBtns');
	allLevels.forEach(function(lv,i){
		var btn = document.createElement('button');
		btn.className = 'tog'+(i===0?' active-period':'');
		btn.textContent = lv.interval;
		btn.addEventListener('click',function(){switchLevel(i);});
		periodBtns.appendChild(btn);
	});

	// 嵌套信息
	if(data.nesting){
		var n = data.nesting;
		document.getElementById('statusBar').innerHTML =
			'区间套: <b>'+(n.levels||[]).join('→')+'</b> 精度 <b>'+(n.accuracy*100).toFixed(0)+'%</b>';
	}

	wrap = document.getElementById('chartWrap');
	chart = createChart(wrap,{
		layout:{background:{color:'#1a1d23'},textColor:'#878d96'},
		grid:{vertLines:{color:'rgba(255,255,255,0.05)'},horzLines:{color:'rgba(255,255,255,0.05)'}},
		crosshair:{mode:0},handleScroll:{vertTouchDrag:false},
		timeScale:{borderColor:'#333842',timeVisible:true,secondsVisible:false},
		rightPriceScale:{borderColor:'#333842',autoScale:true,scaleMargins:{top:0.05,bottom:0.12}},
	});
	series = chart.addSeries(CandlestickSeries,{
		upColor:'#26a69a',downColor:'#ef5350',
		borderUpColor:'#26a69a',borderDownColor:'#ef5350',
		wickUpColor:'#26a69a',wickDownColor:'#ef5350',
		priceLineVisible:false,lastValueVisible:false,
	});

	// Canvas overlay
	var canvas = document.createElement('canvas');
	canvas.style.cssText='position:absolute;top:0;left:0;pointer-events:none;z-index:10;';
	wrap.appendChild(canvas);
	ctx = canvas.getContext('2d');

	// Layer toggle buttons
	document.querySelectorAll('.tog[data-layer]').forEach(function(btn){
		btn.addEventListener('click',function(){
			var layer=this.dataset.layer;
			showLayers[layer]=!showLayers[layer];
			this.classList.toggle('on');
			scheduleDraw();
		});
	});

	resizeCanvas();
	window.addEventListener('resize',function(){
		resizeCanvas();
		chart.applyOptions({width:wrap.clientWidth,height:wrap.clientHeight});
	});

	chart.timeScale().subscribeVisibleTimeRangeChange(function(){scheduleDraw();});
	chart.timeScale().subscribeSizeChange(function(){resizeCanvas();scheduleDraw();});

	switchLevel(0);
}

function switchLevel(idx){
	if(idx<0||idx>=allLevels.length)return;
	currentLevelIdx = idx;
	var lv = allLevels[idx];

	// 高亮当前周期按钮
	var btns = document.getElementById('periodBtns').children;
	for(var i=0;i<btns.length;i++)btns[i].classList.toggle('active-period',i===idx);

	// 更新 K 线
	var kl = lv.merged || [];
	var limit = Math.min(kl.length, MAX_CANDLES);
	klCache = kl.slice(0, limit);

	var candles = klCache.map(function(d){return{time:toSec(d.time),open:d.open,high:d.high,low:d.low,close:d.close};});
	series.setData(candles);
	chart.timeScale().fitContent();

	drawNow();
	scheduleDraw();
	updateStats(lv);
}

function updateStats(lv){
	var el = document.getElementById('stats');
	el.innerHTML = '级别 <b>'+allLevels.length+'</b> 共振 <b>'+(allLevels.length>1?'...':'0')+'</b> 展示 <b>'+lv.interval+'</b> K线 <b>'+klCache.length+'</b> 笔 <b>'+(lv.bis?lv.bis.length:0)+'</b> 线段 <b>'+(lv.segments?lv.segments.length:0)+'</b> 中枢 <b>'+(lv.pivots?lv.pivots.length:0)+'</b> 信号 <b>'+(lv.signals?lv.signals.length:0)+'</b>';
}

function resizeCanvas(){
	var dpr=window.devicePixelRatio||1;
	var w=wrap.clientWidth,h=wrap.clientHeight;
	if(!ctx||!ctx.canvas)return;
	var cv=ctx.canvas;
	cv.width=w*dpr;cv.height=h*dpr;
	cv.style.width=w+'px';cv.style.height=h+'px';
	ctx.setTransform(dpr,0,0,dpr,0,0);
}

var drawPending = false;
function scheduleDraw(){
	if(drawPending)return;
	drawPending=true;
	requestAnimationFrame(function(){drawPending=false;try{drawOverlay();}catch(e){console.error('drawOverlay error',e);}});
}

// 同步直接绘制（首次加载或切换周期时用）
function drawNow(){
	try{drawOverlay();}catch(e){console.error('drawOverlay error',e);}
}

function drawOverlay(){
	ctx.clearRect(0,0,wrap.clientWidth,wrap.clientHeight);
	var ts=chart.timeScale();
	var sp=series.priceToCoordinate.bind(series);
	var tc=ts.timeToCoordinate.bind(ts);
	var lv = allLevels[currentLevelIdx];
	if(!lv)return;

	var kl = klCache;
	function gt(i){if(i>=0&&i<kl.length)return toSec(kl[i].time);return 0;}
	function fld(v,a,b,c){return v[a]??v[b]??v[c]??0;}

	function getIdx(v){return fld(v,'start_index','StartIndex','startIndex');}
	function getEndIdx(v){return fld(v,'end_index','EndIndex','endIndex');}

	// 笔
	if(showLayers.bi&&lv.bis){
		for(var i=0;i<lv.bis.length;i++){
			var b=lv.bis[i];
			var si=getIdx(b),ei=getEndIdx(b);
			var sp_=fld(b,'start_price','StartPrice','startPrice'),ep=fld(b,'end_price','EndPrice','endPrice');
			var dir=fld(b,'direction','Direction','')||'';
			var x1=tc(gt(si)),y1=sp(sp_),x2=tc(gt(ei)),y2=sp(ep);
			if(x1==null||y1==null||x2==null||y2==null)continue;
			ctx.strokeStyle=dir=='up'||dir==='1'?'#5c8aff':'#f5a623';
			ctx.lineWidth=1.8;
			ctx.beginPath();ctx.moveTo(x1,y1);ctx.lineTo(x2,y2);ctx.stroke();
		}
	}

	// 线段
	if(showLayers.seg&&lv.segments){
		for(var i=0;i<lv.segments.length;i++){
			var s=lv.segments[i];
			var si=getIdx(s),ei=getEndIdx(s);
			var x1=tc(gt(si)),y1=sp(kl[si]?kl[si].high:0);
			var x2=tc(gt(ei)),y2=sp(kl[ei]?kl[ei].low:0);
			if(x1==null||y1==null||x2==null||y2==null)continue;
			ctx.strokeStyle='#ab47bc';ctx.lineWidth=2.5;
			ctx.setLineDash([6,3]);ctx.beginPath();ctx.moveTo(x1,y1);ctx.lineTo(x2,y2);ctx.stroke();
			ctx.setLineDash([]);ctx.fillStyle='#ab47bc';ctx.font='10px sans-serif';
			ctx.fillText('S'+i,x1+3,y1>y2?y1+12:y1-8);
		}
	}

	// 中枢
	if(showLayers.pvt&&lv.pivots){
		for(var i=0;i<lv.pivots.length;i++){
			var p=lv.pivots[i];
			var si=getIdx(p),ei=getEndIdx(p);
			var zg=fld(p,'zg','ZG','')||0,zd=fld(p,'zd','ZD','')||0;
			if(!zg||!zd)continue;
			var x1=tc(gt(si)),x2=tc(gt(ei));
			var yZg=sp(zg),yZd=sp(zd);
			if(x1==null||x2==null||yZg==null||yZd==null)continue;
			var rx=x1,rw=Math.max(2,x2-x1),ry=yZg,rh=Math.max(1,yZd-yZg);
			ctx.fillStyle='rgba(255,255,255,0.08)';ctx.fillRect(rx,ry,rw,rh);
			ctx.strokeStyle='rgba(255,255,255,0.30)';ctx.lineWidth=1;
			ctx.setLineDash([3,2]);ctx.strokeRect(rx,ry,rw,rh);ctx.setLineDash([]);
			ctx.fillStyle='rgba(255,255,255,0.55)';ctx.font='10px sans-serif';ctx.fillText('ZS',rx+3,ry-3);
		}
	}

	// 信号
	if(showLayers.sig&&lv.signals){
		for(var i=0;i<lv.signals.length;i++){
			var s=lv.signals[i];
			var idx=s.index||s.Index||0,pr=s.price||s.Price||0,tp=s.type||s.Type||0;
			var x=tc(gt(idx)),y=sp(pr);
			if(x==null||y==null)continue;
			var isBuy=tp>0,sz=6;
			ctx.fillStyle=isBuy?'#00e676':'#ff1744';ctx.beginPath();
			if(isBuy){ctx.moveTo(x,y-sz);ctx.lineTo(x-sz,y);ctx.lineTo(x+sz,y);}else{ctx.moveTo(x,y+sz);ctx.lineTo(x-sz,y);ctx.lineTo(x+sz,y);}
			ctx.closePath();ctx.fill();
			ctx.fillStyle=isBuy?'#00e676':'#ff1744';ctx.font='bold 9px sans-serif';ctx.textAlign='center';
			ctx.fillText(isBuy?'买':'卖',x,isBuy?y+14:y-6);ctx.textAlign='left';
		}
	}
}

// 在页面准备好后开始调度绘制
setTimeout(function(){scheduleDraw();},100);
setTimeout(function(){scheduleDraw();},300);
setTimeout(function(){scheduleDraw();},600);

loadData().then(function(d){start(d);}).catch(function(e){});
</script>
</body>
</html>`
