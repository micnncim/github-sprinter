package github_sprinter

import (
	"io/ioutil"
	"time"

	"github.com/go-yaml/yaml"
)

const (
	timeFormat = "2006/01/02"
)

var (
	day = time.Hour * 24
)

var daysOfWeek = map[string]time.Weekday{
	"Sunday":    time.Sunday,
	"Monday":    time.Monday,
	"Tuesday":   time.Tuesday,
	"Wednesday": time.Wednesday,
	"Thursday":  time.Thursday,
	"Friday":    time.Friday,
	"Saturday":  time.Saturday,
}

func loadManifest(path string) (*Manifest, error) {
	var m Manifest
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(buf, &m); err != nil {
		return nil, err
	}
	return &m, err
}

type Manifest struct {
	Sprint *Sprint `yaml:"sprint"`
	Repos  []*Repo `yaml:"repos"`
}

type Sprint struct {
	TitleFormat string   `yaml:"title_format"`
	Duration    Duration `yaml:"duration"`
	Terms       []*Term  `yaml:"terms"`
	Ignore      *Ignore  `yaml:"ignore"`
}

func (s *Sprint) GenerateMilestones() ([]*Milestone, error) {
	var milestones []*Milestone
	for _, term := range s.Terms {
		termStartOn, termDueOn, err := term.Parse()
		if err != nil {
			return nil, err
		}

		d, err := s.Duration.Parse()
		if err != nil {
			return nil, err
		}
		startOn, dueOn := termStartOn, termStartOn.Add(d)
		for sid := 1; ; sid++ {
			startOn, dueOn, err = s.Ignore.OmitIgnored(startOn, dueOn, d)
			if err != nil {
				return nil, err
			}

			if dueOn.After(termDueOn) {
				dueOn = termDueOn
				m, err := NewMilestone(sid, s.TitleFormat, startOn, dueOn)
				if err != nil {
					return nil, err
				}
				milestones = append(milestones, m)
				break
			}

			m, err := NewMilestone(sid, s.TitleFormat, startOn, dueOn)
			if err != nil {
				return nil, err
			}
			milestones = append(milestones, m)
			startOn = dueOn.Add(day)
			dueOn = startOn.Add(d).Add(-day)
		}

	}

	return milestones, nil
}

type Duration string

func (d Duration) Parse() (time.Duration, error) {
	return time.ParseDuration(string(d))
}

type Term struct {
	StartOn string `yaml:"start_on"`
	DueOn   string `yaml:"due_on"`
}

func (t *Term) Parse() (startOn, dueOn time.Time, err error) {
	startOn, err = time.Parse(timeFormat, t.StartOn)
	if err != nil {
		return
	}
	dueOn, err = time.Parse(timeFormat, t.DueOn)
	if err != nil {
		return
	}
	return
}

type Ignore struct {
	Terms    []*Term   `yaml:"terms"`
	Weekdays []Weekday `yaml:"edge_weekdays"`
}

func (i *Ignore) OmitIgnored(startOn, dueOn time.Time, duration time.Duration) (validStartOn, validDueOn time.Time, err error) {
	validStartOn, validDueOn = startOn, dueOn
	for _, term := range i.Terms {
		var iso time.Time
		var ido time.Time
		iso, ido, err = term.Parse()
		if err != nil {
			return
		}
		// startOn in ignored term
		if (startOn.After(iso) && startOn.Before(ido)) || startOn.Equal(iso) || startOn.Equal(ido) {
			validStartOn = ido.Add(day)
			validDueOn = validStartOn.Add(duration).Add(-day)
			return
		}
		// dueOn in ignored term
		if (dueOn.After(iso) && dueOn.Before(ido)) || dueOn.Equal(iso) || dueOn.Equal(ido) {
			validDueOn = iso.Add(-day)
			return
		}
		// term includes ignored term
		if startOn.Before(iso) && dueOn.After(ido) {
			validDueOn = iso.Add(-day)
			return
		}
		// ignored term includes term
		if startOn.After(iso) && dueOn.Before(ido) {
			validStartOn = ido.Add(day)
			validDueOn = validStartOn.Add(duration).Add(-day)
			return
		}
	}

	for _, w := range i.Weekdays {
		// if ignored weekday, on++
		if validStartOn.Weekday() == w.Parse() {
			validStartOn = validStartOn.Add(day)
		}
		if validDueOn.Weekday() == w.Parse() {
			validDueOn = validDueOn.Add(day)
		}
	}
	return
}

type Weekday string

func (w Weekday) Parse() time.Weekday {
	return daysOfWeek[string(w)]
}

type Repo struct {
	Name string `yaml:"name"`
}
