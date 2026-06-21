package types

import (
	"time"

	"github.com/gocarina/gocsv"
)

// DateTime 封装 time.Time，实现 gocsv 序列化接口。
// CSV 格式："2006-01-02 15:04:05"；JSON 保持 RFC3339。
type DateTime struct {
	time.Time
}

// MarshalCSV 实现 gocsv.TypeMarshaller。
func (d DateTime) MarshalCSV() (string, error) {
	return d.Format(time.DateTime), nil
}

// UnmarshalCSV 实现 gocsv.TypeUnmarshaller。
func (d *DateTime) UnmarshalCSV(csv string) error {
	t, err := time.Parse(time.DateTime, csv)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// MarshalJSON 保持 RFC3339 JSON 输出。
func (d DateTime) MarshalJSON() ([]byte, error) {
	return d.Time.MarshalJSON()
}

// UnmarshalJSON 保持 RFC3339 JSON 解析。
func (d *DateTime) UnmarshalJSON(data []byte) error {
	return d.Time.UnmarshalJSON(data)
}

// Ensure DateTime implements gocsv interfaces at compile time.
var (
	_ gocsv.TypeMarshaller   = DateTime{}
	_ gocsv.TypeUnmarshaller = &DateTime{}
)
