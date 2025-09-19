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
	if len(objects) != 0 {
		_, _ = fmt.Fprintf(os.Stdout, "%s - %s:\n", tr.W("Descending order by total size"), tr.W("All Branches and Tags"))
	}
	su.draw(os.Stdout)
	return nil
}

func Az(ctx context.Context, repoPath string, limit int64, fullPath bool) error {
	objects := make(map[string]int64)
	filter, err := deflect.NewFilter(repoPath, git.HashFormatOK(repoPath), &deflect.FilterOption{
		Limit: limit,
		Rejector: func(oid string, size int64) error {
			objects[oid] = size
			return nil
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "hot az: new filter: %v\n", err)
		return err
	}
	if err := filter.Execute(nil); err != nil {
		fmt.Fprintf(os.Stderr, "hot az: check large file: %v\n", err)
		return err
	}
	_ = showHugeObjects(ctx, repoPath, objects, fullPath)
	fmt.Fprintf(os.Stderr, "%s%s\n", tr.W("Size: "), blue(strengthen.FormatSize(filter.Size())))
	return nil
}
