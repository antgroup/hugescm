package main

import (
	"time"

	"github.com/antgroup/hugescm/pkg/progress"
)

func main() {
	b := progress.NewBar("init", 100, false)
	for i := 0; i < 100; i++ {
		time.Sleep(time.Millisecond * 100)
		b.Add(1)
	}
	b.Finish()
}
