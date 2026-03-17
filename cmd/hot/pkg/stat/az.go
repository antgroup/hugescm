package stat

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/deflect"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
)

func showHugeObjects(ctx context.Context, repoPath string, objects map[string]int64, fullPath bool) error {
	su := newSummer(fullPath)
	psArgs := []string{"rev-list", "--objects", "--all"}
	if err := su.resolveName(ctx, repoPath, objects, psArgs, su.printName); err != nil {
		fmt.Fprintf(os.Stderr, "hot az: resolve file name error: %v\n", err)
		return err
	}
	if err := su.drawInteractive(fmt.Sprintf("%s - %s", tr.W("Descending order by total size"), tr.W("All Branches and Tags"))); err != nil {
		return err
	}
	return nil
}

func Az(ctx context.Context, repoPath string, limit int64, fullPath bool) error {
	objects := make(map[string]int64)
	au := deflect.NewAuditor(repoPath, git.HashFormatOK(repoPath), &deflect.Option{
		Limit: limit,
		OnOversized: func(oid string, size int64) error {
			objects[oid] = size
			return nil
		},
	})
	if err := au.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "hot az: check large file: %v\n", err)
		return err
	}
	_ = showHugeObjects(ctx, repoPath, objects, fullPath)
	fmt.Fprintf(os.Stderr, "%s%s\n", tr.W("Size: "), blue(strengthen.FormatSize(au.Size())))
	return nil
}
