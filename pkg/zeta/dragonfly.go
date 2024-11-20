// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/progress"
	"github.com/antgroup/hugescm/pkg/transport"
)

// url="http://oss-x.alipay.com/path/to/data?Expires=1679985949&OSSAccessKeyId=test&Signature=test"
// dfget --filter "Expires&Signature" \
//     -u "$url" \
//     --output /path/to/output

// dfget

const (
	ENV_ZETA_EXTENSION_DRAGONFLY_GET = "ZETA_EXTENSION_DRAGONFLY_GET"
)

func LookupDragonflyGet() (string, error) {
	if dfget, ok := os.LookupEnv(ENV_ZETA_EXTENSION_DRAGONFLY_GET); ok {
		if d, err := exec.LookPath(dfget); err == nil {
			return d, nil
		}
	}
	dfget, err := exec.LookPath("dfget")
	if err != nil {
		return "", err
	}
	return dfget, nil
}

func (r *Repository) doDragonflyGetOne(ctx context.Context, dfget string, stdout, stderr io.Writer, o *transport.Representation) error {
	oid := plumbing.NewHash(o.OID)
	saveTo := r.odb.JoinPart(oid)
	psArgs := make([]string, 0, 8)
	psArgs = append(psArgs,
		"-u", o.Href,
		"--filter", "Expires&Signature",
		"--output", saveTo)
	// https://github.com/dragonflyoss/Dragonfly2/blob/main/cmd/dfget/cmd/root.go
	// url header, eg: --header='Accept: *' --header='Host: abc'
	for h, v := range o.Header {
		psArgs = append(psArgs, fmt.Sprintf("--header=%s: %s", h, v))
	}
	if !r.quiet {
		// After testing, the download progress bar of dfget seems to have no effect.
		psArgs = append(psArgs, "--show-progress")
	}
	cmd := exec.CommandContext(ctx, dfget, psArgs...)
	cmd.Stderr = stdout
	cmd.Stdout = stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return r.odb.ValidatePart(saveTo, oid)
}

func dragonflyOutput(s string) string {
	sv := strings.Split(s, "\n")
	return strings.Join(sv, "\ndfget: ")
}

func (r *Repository) doDragonflyParallelGet(ctx context.Context, dfget string, objects []*transport.Representation, bar *progress.Indicators) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(objects))
	for _, o := range objects {
		wg.Add(1)
		go func(ro *transport.Representation) {
			defer wg.Done()
			if ro.IsExpired() {
				fmt.Fprintf(os.Stderr, "object '%s' download link expired at: %v\n", ro.OID, ro.ExpiresAt)
				errs <- nil
				return
			}
			stderr := command.NewStderr()
			if err := r.doDragonflyGetOne(ctx, dfget, nil, stderr, ro); err != nil {
				fmt.Fprintf(os.Stderr, "\x1b[2K\rDownload %s url: %s [Dragonfly P2P] error: \ndfget: %s\n", ro.OID, ro.Href, dragonflyOutput(stderr.String()))
				errs <- err
				return
			}
			bar.Add(1)
			errs <- nil
		}(o.Copy())
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) doDragonflyGet(ctx context.Context, dfget string, objects []*transport.Representation, concurrent int, bar *progress.Indicators) error {
	for len(objects) > 0 {
		g := min(concurrent, len(objects))
		if err := r.doDragonflyParallelGet(ctx, dfget, objects[:g], bar); err != nil {
			return err
		}
		objects = objects[g:]
	}
	return nil
}

func (r *Repository) dragonflyGet(ctx context.Context, objects []*transport.Representation) error {
	if len(objects) == 0 {
		return nil
	}
	concurrent := r.ConcurrentTransfers()
	r.DbgPrint("concurrent transfers %d", concurrent)
	dfget, err := LookupDragonflyGet()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lookup dfget %s\n", err)
		return err
	}
	if concurrent <= 1 || len(objects) == 1 {
		for i, o := range objects {
			if o.IsExpired() {
				fmt.Fprintf(os.Stderr, "object '%s' download link expired at: %v\n", o.OID, o.ExpiresAt)
				return nil
			}
			start := time.Now()
			if err := r.doDragonflyGetOne(ctx, dfget, os.Stdout, os.Stderr, o); err != nil {
				fmt.Fprintf(os.Stderr, "dfget download %s %s\n", o.OID, err)
				return err
			}
			fmt.Fprintf(os.Stderr, "\x1b[2K\r\x1b[38;2;72;198;239m[%d/%d]\x1b[0m Download %s completed, size: %s %s: %v [Dragonfly P2P]\n", i+1, len(objects), o.OID, strengthen.HumanateSize(o.CompressedSize), W("time spent"), time.Since(start).Truncate(time.Millisecond))
		}
		return nil
	}
	bar := progress.NewIndicators("Batch download files", "Batch download files completed", uint64(len(objects)), r.quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	if err := r.doDragonflyGet(ctx, dfget, objects, concurrent, bar); err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	return nil
}
