package types

import "math"

// ──────────────────────────────────────────────
// 全局数值常量
// ──────────────────────────────────────────────

// ForceEpsilon 力度计算的零值基线。
// 所有力度指标（MACD 面积、峰值等）在零值时返回此量级，
// 下游的"力度是否为零"判断应统一使用此常量，避免跨包 epsilon 不一致导致假背驰。
const ForceEpsilon = 1e-12

// DivergenceSentinelThreshold 背驰率哨兵阈值。
// 当 BspDivergenceRate 设为 +Inf 或超过此值时，视为关闭背驰率过滤（任何力度衰减都视为背驰）。
const DivergenceSentinelThreshold = 100.0

// ──────────────────────────────────────────────
// 辅助数学函数
// ──────────────────────────────────────────────

func Max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func Min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// SafeDivide 安全除法，分母接近 0 时返回 fallback。
func SafeDivide(num, den, fallback float64) float64 {
	if Abs(den) < ForceEpsilon {
		return fallback
	}
	return num / den
}

// IsPriceValid 检查价格是否为合法正数。
func IsPriceValid(p float64) bool {
	return !math.IsNaN(p) && !math.IsInf(p, 0) && p > 0
}
