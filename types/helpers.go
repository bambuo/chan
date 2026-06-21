package types

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
	if Abs(den) < 1e-12 {
		return fallback
	}
	return num / den
}

// IsPriceValid 检查价格是否为合法正数。
func IsPriceValid(p float64) bool {
	return !isNaN(p) && !isInf(p) && p > 0
}

func isNaN(f float64) bool {
	return f != f
}

func isInf(f float64) bool {
	return f > 1e300 || f < -1e300
}
