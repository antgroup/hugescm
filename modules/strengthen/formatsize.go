package strengthen

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Port from: https://github.com/docker/go-units/blob/master/size.go

import (
	"fmt"
)

const (
	sizeByteBase = 1024.0
)

var (
	sizeLists = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
)

func getSizeAndUnit(size float64) (float64, string) {
	i := 0
	unitsLimit := len(sizeLists) - 1
	for size >= sizeByteBase && i < unitsLimit {
		size /= sizeByteBase
		i++
	}
	return size, sizeLists[i]
}

func formatBytes(size float64) string {
	size, unit := getSizeAndUnit(size)
	return fmt.Sprintf("%.4g %s", size, unit)
}

func FormatSize(s int64) string {
	return formatBytes(float64(s))
}

func FormatSizeU(s uint64) string {
	return formatBytes(float64(s))
}
