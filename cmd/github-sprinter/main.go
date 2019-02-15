package main

import (
	"context"
	"flag"
	"log"

	"golang.org/x/sync/errgroup"

	sprinter "github.com/micnncim/github-sprinter"
)

func main() {
	var (
		manifest = flag.String("manifest", "sprint.yaml", "manifest yaml file")
		dryRun   = flag.Bool("dry-run", false, "dry run flag")
		update   = flag.Bool("update", false, "update flag (destructive change)")
	)
	flag.Parse()

	ctx := context.Background()
	sprinter, err := sprinter.NewSprinter(ctx, *manifest, *dryRun, *update)
	if err != nil {
		log.Fatal(err)
	}
	eg := errgroup.Group{}
	for _, repo := range sprinter.Manifest.Repos {
		repo := repo
		eg.Go(func() error {
			return sprinter.ApplyManifest(ctx, repo)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}
}
