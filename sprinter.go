package github_sprinter

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/google/go-github/v24/github"
)

type githubClient struct {
	*github.Client

	dryRun bool
	update bool

	common service

	Milestone *MilestoneService
}

type MilestoneService service

type service struct {
	client *githubClient
}

type Sprinter struct {
	github   *githubClient
	Manifest *Manifest
}

func (s *MilestoneService) Create(ctx context.Context, owner, repo string, milestone *Milestone) error {
	_, dueOn, err := milestone.ParseDate()
	if err != nil {
		return err
	}
	ghMilestone := &github.Milestone{
		Title:       github.String(milestone.Title),
		State:       github.String(milestone.State),
		Description: github.String(milestone.Description),
		DueOn:       &dueOn,
	}
	log.Printf("create %q in %s/%s", milestone.Title, owner, repo)
	if s.client.dryRun {
		return nil
	}
	if _, _, err := s.client.Issues.CreateMilestone(ctx, owner, repo, ghMilestone); err != nil {
		return err
	}
	return nil
}

func (s *MilestoneService) List(ctx context.Context, owner, repo string) ([]*Milestone, error) {
	ghMilestones, _, err := s.client.Issues.ListMilestones(
		ctx,
		owner,
		repo,
		&github.MilestoneListOptions{
			ListOptions: github.ListOptions{PerPage: 10},
		},
	)
	if err != nil {
		return nil, err
	}

	var milestones []*Milestone
	for _, m := range ghMilestones {
		milestones = append(milestones, &Milestone{
			Number:      *m.Number,
			Title:       *m.Title,
			State:       *m.State,
			Description: *m.Description,
			DueOn:       m.DueOn.Format(timeFormat),
		})
	}

	return milestones, nil
}

func (s *MilestoneService) Delete(ctx context.Context, owner, repo string, milestone *Milestone) error {
	log.Printf("delete %q in %s/%s", milestone.Title, owner, repo)
	if s.client.dryRun {
		return nil
	}

	_, err := s.client.Issues.DeleteMilestone(ctx, owner, repo, milestone.Number)
	if err != nil {
		return err
	}
	return nil
}

func NewSprinter(ctx context.Context, configPath string, dryRun, update bool) (*Sprinter, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is missing")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	m, err := loadManifest(configPath)
	if err != nil {
		return nil, err
	}

	gc := &githubClient{
		Client: client,
		dryRun: dryRun,
		update: update,
	}
	gc.common.client = gc
	gc.Milestone = (*MilestoneService)(&gc.common)
	return &Sprinter{
		github:   gc,
		Manifest: m,
	}, nil
}

func (s *Sprinter) ApplyManifest(ctx context.Context, repository *Repo) error {
	slugs := strings.Split(repository.Name, "/")
	if len(slugs) != 2 {
		return fmt.Errorf("repository name %q is invalid", repository.Name)
	}
	owner, repo := slugs[0], slugs[1]

	if s.github.update {
		// delete all milestones (state="open")
		ms, err := s.github.Milestone.List(ctx, owner, repo)
		if err != nil {
			return err
		}
		for _, m := range ms {
			if err := s.github.Milestone.Delete(ctx, owner, repo, m); err != nil {
				return err
			}
		}
	}

	milestones, err := s.Manifest.Sprint.GenerateMilestones()
	if err != nil {
		return err
	}
	for _, m := range milestones {
		if err := s.github.Milestone.Create(ctx, owner, repo, m); err != nil {
			return err
		}
	}

	return nil
}
