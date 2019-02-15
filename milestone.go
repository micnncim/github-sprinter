package github_sprinter

import (
	"bytes"
	"text/template"
	"time"
)

type Milestone struct {
	Number      int
	SID         int // Sprint ID
	Title       string
	State       string
	Description string
	StartOn     string
	DueOn       string
}

func NewMilestone(sid int, titleFmt string, startOn, dueOn time.Time) (*Milestone, error) {
	tmpl, err := template.New("title").Parse(titleFmt)
	if err != nil {
		return nil, err
	}
	m := &Milestone{
		SID:     sid,
		State:   "open",
		StartOn: startOn.Format(timeFormat),
		DueOn:   dueOn.Format(timeFormat),
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, m); err != nil {
		return nil, err
	}
	m.Title = buf.String()
	return m, nil
}

func (m *Milestone) ParseDate() (startOn, dueOn time.Time, err error) {
	startOn, err = time.Parse(timeFormat, m.StartOn)
	if err != nil {
		return
	}
	dueOn, err = time.Parse(timeFormat, m.DueOn)
	if err != nil {
		return
	}
	return
}
