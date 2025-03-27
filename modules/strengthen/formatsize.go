package strengthen

import (
	"fmt"
	"math"
)

var (
	sizeList = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
)

func logN(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

func formatBytes(s uint64, base float64) string {
	if s < 10 {
		return fmt.Sprintf("%d B", s)
	}
	e := math.Floor(logN(float64(s), base))
	suffix := sizeList[int(e)]
	val := math.Floor(float64(s)/math.Pow(base, e)*10+0.5) / 10
	f := "%.0f %s"
	if val < 10 {
		f = "%.1f %s"
	}

	return fmt.Sprintf(f, val, suffix)
}

func FormatSize(s int64) string {
	return formatBytes(uint64(s), 1024)
}

func FormatSizeU(s uint64) string {
	return formatBytes(s, 1024)
}
