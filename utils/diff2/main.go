package main

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/diferenco/color"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: %s file1 file2\n", os.Args[0])
		return
	}
	fd1, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file %s error: %v\n", os.Args[1], err)
		return
	}
	defer fd1.Close()
	fd2, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file %s error: %v\n", os.Args[2], err)
		return
	}
	defer fd2.Close()

	u, err := diferenco.DoUnified(context.Background(), &diferenco.Options{
		From: &diferenco.File{
			Path: os.Args[1],
		},
		To: &diferenco.File{
			Path: os.Args[2],
		},
		R1: fd1,
		R2: fd2,
	})
	if err != nil {
		return
	}
	e := diferenco.NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*diferenco.Unified{u})

}
