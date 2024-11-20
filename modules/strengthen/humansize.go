package strengthen

import (
	"fmt"
	"math"
)

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

func humanateBytes(s uint64, base float64, sizes []string) string {
	if s < 10 {
		return fmt.Sprintf("%d B", s)
	}
	e := math.Floor(logn(float64(s), base))
	suffix := sizes[int(e)]
	val := math.Floor(float64(s)/math.Pow(base, e)*10+0.5) / 10
	f := "%.0f %s"
	if val < 10 {
		f = "%.1f %s"
	}

	return fmt.Sprintf(f, val, suffix)
}

var (
	sizes = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
)

func HumanateSize(s int64) string {
	return humanateBytes(uint64(s), 1024, sizes)
}

func HumanateSizeU(s uint64) string {
	return humanateBytes(s, 1024, sizes)
}
