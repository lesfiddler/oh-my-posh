package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jandedobbeleer/oh-my-posh/src/cache"
	"github.com/jandedobbeleer/oh-my-posh/src/color"
	"github.com/jandedobbeleer/oh-my-posh/src/properties"
	"github.com/jandedobbeleer/oh-my-posh/src/runtime"
	"github.com/jandedobbeleer/oh-my-posh/src/template"

	c "golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SegmentStyle the style of segment, for more information, see the constants
type SegmentStyle string

func (s *SegmentStyle) resolve(context any) SegmentStyle {
	txtTemplate := &template.Text{
		Context: context,
	}

	txtTemplate.Template = string(*s)
	value, err := txtTemplate.Render()

	// default to Plain
	if err != nil || len(value) == 0 {
		return Plain
	}

	return SegmentStyle(value)
}

type Segment struct {
	writer SegmentWriter
	env    runtime.Environment

	Properties             properties.Map `json:"properties,omitempty" toml:"properties,omitempty"`
	Cache                  *cache.Config  `json:"cache,omitempty" toml:"cache,omitempty"`
	Filler                 string         `json:"filler,omitempty" toml:"filler,omitempty"`
	styleCache             SegmentStyle
	name                   string
	LeadingDiamond         string         `json:"leading_diamond,omitempty" toml:"leading_diamond,omitempty"`
	TrailingDiamond        string         `json:"trailing_diamond,omitempty" toml:"trailing_diamond,omitempty"`
	Template               string         `json:"template,omitempty" toml:"template,omitempty"`
	Foreground             color.Ansi     `json:"foreground" toml:"foreground"`
	TemplatesLogic         template.Logic `json:"templates_logic,omitempty" toml:"templates_logic,omitempty"`
	PowerlineSymbol        string         `json:"powerline_symbol,omitempty" toml:"powerline_symbol,omitempty"`
	Background             color.Ansi     `json:"background" toml:"background"`
	Alias                  string         `json:"alias,omitempty" toml:"alias,omitempty"`
	Type                   SegmentType    `json:"type,omitempty" toml:"type,omitempty"`
	Style                  SegmentStyle   `json:"style,omitempty" toml:"style,omitempty"`
	LeadingPowerlineSymbol string         `json:"leading_powerline_symbol,omitempty" toml:"leading_powerline_symbol,omitempty"`
	ForegroundTemplates    template.List  `json:"foreground_templates,omitempty" toml:"foreground_templates,omitempty"`
	Tips                   []string       `json:"tips,omitempty" toml:"tips,omitempty"`
	BackgroundTemplates    template.List  `json:"background_templates,omitempty" toml:"background_templates,omitempty"`
	MinWidth               int            `json:"min_width,omitempty" toml:"min_width,omitempty"`
	MaxWidth               int            `json:"max_width,omitempty" toml:"max_width,omitempty"`
	Duration               time.Duration  `json:"-" toml:"-"`
	NameLength             int            `json:"-" toml:"-"`
	Interactive            bool           `json:"interactive,omitempty" toml:"interactive,omitempty"`
	Enabled                bool           `json:"-" toml:"-"`
	Newline                bool           `json:"newline,omitempty" toml:"newline,omitempty"`
	InvertPowerline        bool           `json:"invert_powerline,omitempty" toml:"invert_powerline,omitempty"`
}

func (segment *Segment) Name() string {
	if len(segment.name) != 0 {
		return segment.name
	}

	name := segment.Alias
	if len(name) == 0 {
		name = c.Title(language.English).String(string(segment.Type))
	}

	segment.name = name
	return name
}

func (segment *Segment) Execute(env runtime.Environment) {
	// segment timings for debug purposes
	var start time.Time
	if env.Flags().Debug {
		start = time.Now()
		segment.NameLength = len(segment.Name())
		defer func() {
			segment.Duration = time.Since(start)
		}()
	}

	err := segment.MapSegmentWithWriter(env)
	if err != nil || !segment.shouldIncludeFolder() {
		return
	}

	segment.env.DebugF("segment: %s", segment.Name())

	if segment.isToggled() {
		return
	}

	if segment.restoreCache() {
		return
	}

	if shouldHideForWidth(segment.env, segment.MinWidth, segment.MaxWidth) {
		return
	}

	if segment.writer.Enabled() {
		segment.Enabled = true
		env.TemplateCache().AddSegmentData(segment.Name(), segment.writer)
	}
}

func (segment *Segment) Render() {
	if !segment.Enabled {
		return
	}

	text := segment.string()
	segment.Enabled = len(strings.ReplaceAll(text, " ", "")) > 0

	if !segment.Enabled {
		segment.env.TemplateCache().RemoveSegmentData(segment.Name())
		return
	}

	segment.writer.SetText(text)
	segment.setCache()
}

func (segment *Segment) Text() string {
	return segment.writer.Text()
}

func (segment *Segment) SetText(text string) {
	segment.writer.SetText(text)
}

func (segment *Segment) ResolveForeground() color.Ansi {
	if len(segment.ForegroundTemplates) != 0 {
		match := segment.ForegroundTemplates.FirstMatch(segment.writer, segment.Foreground.String())
		segment.Foreground = color.Ansi(match)
	}

	return segment.Foreground
}

func (segment *Segment) ResolveBackground() color.Ansi {
	if len(segment.BackgroundTemplates) != 0 {
		match := segment.BackgroundTemplates.FirstMatch(segment.writer, segment.Background.String())
		segment.Background = color.Ansi(match)
	}

	return segment.Background
}

func (segment *Segment) ResolveStyle() SegmentStyle {
	if len(segment.styleCache) != 0 {
		return segment.styleCache
	}

	segment.styleCache = segment.Style.resolve(segment.writer)

	return segment.styleCache
}

func (segment *Segment) IsPowerline() bool {
	style := segment.ResolveStyle()
	return style == Powerline || style == Accordion
}

func (segment *Segment) HasEmptyDiamondAtEnd() bool {
	if segment.ResolveStyle() != Diamond {
		return false
	}

	return len(segment.TrailingDiamond) == 0
}

func (segment *Segment) hasCache() bool {
	return segment.Cache != nil && !segment.Cache.Duration.IsEmpty()
}

func (segment *Segment) isToggled() bool {
	toggles, OK := segment.env.Session().Get(cache.TOGGLECACHE)
	if !OK || len(toggles) == 0 {
		return false
	}

	list := strings.Split(toggles, ",")
	for _, toggle := range list {
		if SegmentType(toggle) == segment.Type || toggle == segment.Alias {
			segment.env.DebugF("segment toggled off: %s", segment.Name())
			return true
		}
	}

	return false
}

func (segment *Segment) restoreCache() bool {
	if !segment.hasCache() {
		return false
	}

	data, OK := segment.env.Session().Get(segment.cacheKey())
	if !OK {
		return false
	}

	err := json.Unmarshal([]byte(data), &segment.writer)
	if err != nil {
		segment.env.Error(err)
	}

	segment.Enabled = true
	segment.env.TemplateCache().AddSegmentData(segment.Name(), segment.writer)

	return true
}

func (segment *Segment) setCache() {
	if !segment.hasCache() {
		return
	}

	data, err := json.Marshal(segment.writer)
	if err != nil {
		segment.env.Error(err)
		return
	}

	segment.env.Session().Set(segment.cacheKey(), string(data), segment.Cache.Duration)
}

func (segment *Segment) cacheKey() string {
	format := "segment_cache_%s"
	switch segment.Cache.Strategy {
	case cache.Session:
		return fmt.Sprintf(format, segment.Name())
	case cache.Folder:
		fallthrough
	default:
		return fmt.Sprintf(format, strings.Join([]string{segment.Name(), segment.folderKey()}, "_"))
	}
}

func (segment *Segment) folderKey() string {
	ctx, ok := segment.writer.(cache.Context)
	if !ok {
		return segment.env.Pwd()
	}

	if key, OK := ctx.CacheKey(); OK {
		return key
	}

	return segment.env.Pwd()
}

func (segment *Segment) string() string {
	if len(segment.Template) == 0 {
		segment.Template = segment.writer.Template()
	}

	tmpl := &template.Text{
		Template: segment.Template,
		Context:  segment.writer,
	}

	text, err := tmpl.Render()
	if err != nil {
		return err.Error()
	}

	return text
}

func (segment *Segment) shouldIncludeFolder() bool {
	if segment.env == nil {
		return true
	}

	cwdIncluded := segment.cwdIncluded()
	cwdExcluded := segment.cwdExcluded()

	return cwdIncluded && !cwdExcluded
}

func (segment *Segment) cwdIncluded() bool {
	value, ok := segment.Properties[properties.IncludeFolders]
	if !ok {
		// IncludeFolders isn't specified, everything is included
		return true
	}

	list := properties.ParseStringArray(value)

	if len(list) == 0 {
		// IncludeFolders is an empty array, everything is included
		return true
	}

	return segment.env.DirMatchesOneOf(segment.env.Pwd(), list)
}

func (segment *Segment) cwdExcluded() bool {
	value, ok := segment.Properties[properties.ExcludeFolders]
	if !ok {
		return false
	}

	list := properties.ParseStringArray(value)
	return segment.env.DirMatchesOneOf(segment.env.Pwd(), list)
}
