// Package features 提供缠论信号的 ML 特征提取。
//
// 对齐 Python ChanModel/Features.py + BuySellPoint/BSPointList.py 的 12 个内置特征。
// 每个信号类型（T1/T1P/T2/T2S/T3）有其专用的特征集。
package features

import "github.com/bambuo/chan/types"

// ── 特征常量 ──

const (
	KeyBiAmp          = "bsp_bi_amp"         // 信号所在笔的振幅（通用）
	KeyDivergenceRate = "divergence_rate"    // MACD 背驰率（T1/T1P）
	KeyZsCnt          = "zs_cnt"             // 中枢数量（T1）
	KeyBsp1BiAmp      = "bsp1_bi_amp"        // 最后一笔振幅（T1P）
	KeyBsp2Retrace    = "bsp2_retrace_rate"  // 回撤比率（T2）
	KeyBsp2BreakAmp   = "bsp2_break_bi_amp"  // 突破笔振幅（T2）
	KeyBsp2BiAmp      = "bsp2_bi_amp"        // 二买/二卖笔振幅（T2）
	KeyBsp2sRetrace   = "bsp2s_retrace_rate" // 类二买回撤（T2S）
	KeyBsp2sBreakAmp  = "bsp2s_break_bi_amp" // 类二买突破振幅（T2S）
	KeyBsp2sBiAmp     = "bsp2s_bi_amp"       // 类二买笔振幅（T2S）
	KeyBsp2sLv        = "bsp2s_lv"           // 类二买级别（T2S）
	KeyBsp3ZsHeight   = "bsp3_zs_height"     // 中枢相对高度（T3）
	KeyBsp3BiAmp      = "bsp3_bi_amp"        // 三买/三卖笔振幅（T3）
)

// ── 特征提取函数 ──

// Common 提取通用特征：信号所在笔的振幅。
func Common(bi types.Bi) map[string]float64 {
	return map[string]float64{KeyBiAmp: bi.Amp()}
}

// T1 提取一买/一卖特征。
func T1(divergenceRate float64, zsCnt int) map[string]float64 {
	return map[string]float64{
		KeyDivergenceRate: divergenceRate,
		KeyZsCnt:          float64(zsCnt),
	}
}

// T1P 提取盘背一买/一卖特征。
func T1P(divergenceRate, lastBiAmp float64) map[string]float64 {
	return map[string]float64{
		KeyDivergenceRate: divergenceRate,
		KeyBsp1BiAmp:      lastBiAmp,
	}
}

// T2 提取二买/二卖特征。
func T2(retraceRate, breakBiAmp, b2BiAmp float64) map[string]float64 {
	return map[string]float64{
		KeyBsp2Retrace:  retraceRate,
		KeyBsp2BreakAmp: breakBiAmp,
		KeyBsp2BiAmp:    b2BiAmp,
	}
}

// T2S 提取类二买/类二卖特征。
func T2S(retraceRate, breakBiAmp, biAmp float64, lv int) map[string]float64 {
	return map[string]float64{
		KeyBsp2sRetrace:  retraceRate,
		KeyBsp2sBreakAmp: breakBiAmp,
		KeyBsp2sBiAmp:    biAmp,
		KeyBsp2sLv:       float64(lv),
	}
}

// T3 提取三买/三卖特征。
func T3(zsZG, zsZD, biAmp float64) map[string]float64 {
	height := 0.0
	if zsZD > 0 {
		height = (zsZG - zsZD) / zsZD
	}
	return map[string]float64{
		KeyBsp3ZsHeight: height,
		KeyBsp3BiAmp:    biAmp,
	}
}

// Merge 合并多个特征 map 为一个。
func Merge(maps ...map[string]float64) map[string]float64 {
	result := make(map[string]float64)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
