package reader

import (
	"regexp"
	"unicode/utf8"
)

var chapterPath = regexp.MustCompile("^/(story/[a-z0-9-]+|work/[a-z0-9-]+/chapter/[a-z0-9-]+)$")
var paragraph = regexp.MustCompile("^[a-zA-Z0-9_-]{0,160}$")

type Entry struct {
	Href       string `json:"href"`
	Title      string `json:"title"`
	Work       string `json:"work"`
	RecordedAt int64  `json:"recordedAt"`
	Percent    int    `json:"percent"`
	Anchor     string `json:"anchor"`
	Excerpt    string `json:"excerpt"`
}

func (e Entry) Valid() bool {
	return len(e.Href) <= 600 && chapterPath.MatchString(e.Href) &&
		e.Title != "" && utf8.RuneCountInString(e.Title) <= 512 &&
		e.Work != "" && utf8.RuneCountInString(e.Work) <= 512 &&
		e.RecordedAt >= 0 && e.Percent >= 0 && e.Percent <= 100 &&
		paragraph.MatchString(e.Anchor) && utf8.RuneCountInString(e.Excerpt) <= 200
}
