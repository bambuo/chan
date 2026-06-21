package features

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/bambuo/chan/types"
)

// ExportToLibSVM 将信号列表导出为 LIBSVM 格式文件及 feature.meta。
//
// 对齐 Python strategy_demo5.py 的导出逻辑：
//   - feature.libsvm: "label idx1:val1 idx2:val2 ..."
//   - feature.meta: json 文件，feature_name → column_index
//
// labels 长度必须与 signals 相同，对应每个信号的标签（1=正样本，0=负样本）。
// outputPath 是输出路径前缀（不含扩展名），会生成 {outputPath}.libsvm 和 {outputPath}.meta。
func ExportToLibSVM(signals []types.Signal, labels []int, outputPath string) error {
	if len(signals) == 0 {
		return fmt.Errorf("empty signals")
	}
	if len(signals) != len(labels) {
		return fmt.Errorf("signals[%d] and labels[%d] length mismatch", len(signals), len(labels))
	}

	// 收集所有特征名并构建 meta 映射
	meta := make(map[string]int) // feature_name → column_index
	for _, sig := range signals {
		for k := range sig.Features {
			if _, ok := meta[k]; !ok {
				meta[k] = len(meta)
			}
		}
	}

	// 保存 meta 文件
	metaPath := outputPath + ".meta"
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if err := os.WriteFile(metaPath, metaJSON, 0644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}

	// 保存 LIBSVM 文件
	libsvmPath := outputPath + ".libsvm"
	f, err := os.Create(libsvmPath)
	if err != nil {
		return fmt.Errorf("create libsvm: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}

	for i, sig := range signals {
		// label
		line := fmt.Sprintf("%d", labels[i])
		// 将 features 按 meta 顺序排列
		sort.Strings(keys)
		for _, k := range keys {
			if v, ok := sig.Features[k]; ok {
				line += fmt.Sprintf(" %d:%.6f", meta[k], v)
			} else {
				// 缺失特征用 -999999999
				line += fmt.Sprintf(" %d:-999999999", meta[k])
			}
		}
		line += "\n"
		if _, err := w.WriteString(line); err != nil {
			return fmt.Errorf("write line %d: %w", i, err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	return nil
}
