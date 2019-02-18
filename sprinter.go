package github_sprinter

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"

	"github.com/google/go-github/github"
)

type githubClient struct {
	*github.Client

	dryRun bool
	update bool

	common service

	Milestone *MilestoneService
	Issue     *IssueService
}

type MilestoneService service

type IssueService service

type service struct {
	client *githubClient
}

type Sprinter struct {
	github   *githubClient
	Manifest *Manifest
}

type Issue struct {
	Title string
	URL   string
}

func (s *MilestoneService) Create(ctx context.Context, owner, repo string, milestone *Milestone) error {
	fmt.Printf("create %q in %s/%s\n", milestone.Title, owner, repo)
	if s.client.dryRun {
		return nil
	}
	_, dueOn, err := milestone.ParseDate()
	if err != nil {
		return err
	}
	dueOn = dueOn.Add(day)
	ghMilestone := &github.Milestone{
		Title:       github.String(milestone.Title),
		State:       github.String(milestone.State),
		Description: github.String(milestone.Description),
		DueOn:       &dueOn,
	}
	if _, _, err := s.client.Issues.CreateMilestone(ctx, owner, repo, ghMilestone); err != nil {
		return err
	}
	return nil
}

func (s *MilestoneService) List(ctx context.Context, owner, repo string) ([]*Milestone, error) {
	opt := github.ListOptions{PerPage: 10}
	var milestones []*Milestone

	for {
		ghMilestones, resp, err := s.client.Issues.ListMilestones(
			ctx,
			owner,
			repo,
			&github.MilestoneListOptions{
				ListOptions: opt,
			},
		)
		if err != nil {
			return nil, err
		}

		for _, ghMilestone := range ghMilestones {
			description := ""
			if ghMilestones != nil {
				description = *ghMilestone.Description
			}
			milestones = append(milestones, &Milestone{
				Number:      *ghMilestone.Number,
				Title:       *ghMilestone.Title,
				State:       *ghMilestone.State,
				Description: description,
				DueOn:       ghMilestone.DueOn.Format(timeFormat),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return milestones, nil
}

func (s *MilestoneService) Delete(ctx context.Context, owner, repo string, milestone *Milestone) error {
	fmt.Printf("delete %q in %s/%s\n", milestone.Title, owner, repo)
	if s.client.dryRun {
		return nil
	}

	if _, err := s.client.Issues.DeleteMilestone(ctx, owner, repo, milestone.Number); err != nil {
		return err
	}
	return nil
}

func (s *IssueService) ListByMilestone(ctx context.Context, owner, repo string, milestone *Milestone) ([]*Issue, error) {
	opt := github.ListOptions{PerPage: 10}
	var issues []*Issue

	for {
		ghIssues, resp, err := s.client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
			Milestone:   strconv.Itoa(milestone.Number),
			ListOptions: opt,
		})
		if err != nil {
			return nil, err
		}
		for _, ghIssue := range ghIssues {
			issues = append(issues, &Issue{
				Title: *ghIssue.Title,
				URL:   *ghIssue.URL,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return issues, nil
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
	gc.Issue = (*IssueService)(&gc.common)
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

		eg := errgroup.Group{}
		for _, m := range ms {
			m := m
			eg.Go(func() error {
				if err := s.github.Milestone.Delete(ctx, owner, repo, m); err != nil {
					return err
				}
				issues, err := s.github.Issue.ListByMilestone(ctx, owner, repo, m)
				if err != nil {
					return err
				}
				for _, issue := range issues {
					fmt.Printf("  - dislink %q in %q\n", issue.Title, m.Title)
				}

				return nil
			})
			if err := eg.Wait(); err != nil {
				return err
			}
		}
	}

	milestones, err := s.Manifest.Sprint.GenerateMilestones()
	if err != nil {
		return err
	}
	eg := errgroup.Group{}
	for _, m := range milestones {
		eg.Go(func() error {
			return s.github.Milestone.Create(ctx, owner, repo, m)
		})
		if err := eg.Wait(); err != nil {
			return err
		}
	}

	return nil
}
