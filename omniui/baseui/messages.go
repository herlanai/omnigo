package baseui

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func paragraph(id, text string) *Element { return Text(id, text).Wrap(true) }
func Item(id, title, description string, actions ...*Element) *Element {
	children := []*Element{Column(id+"/body", Text(id+"/title", title).FontWeight(600), paragraph(id+"/description", description).Role("note")).Grow(1)}
	children = append(children, actions...)
	row := Row(id+"/row", children...)
	row.centerItems = true
	return Card(id, row).Role("group")
}

type AttachmentStatus string

const (
	AttachmentReady      AttachmentStatus = ""
	AttachmentUploading  AttachmentStatus = "uploading"
	AttachmentProcessing AttachmentStatus = "processing"
	AttachmentError      AttachmentStatus = "error"
)

func Attachment(id, name, description string, status AttachmentStatus, remove func()) *Element {
	actions := []*Element{}
	if status == AttachmentUploading || status == AttachmentProcessing {
		actions = append(actions, Spinner(id+"/busy", string(status)))
	}
	if status == AttachmentError {
		actions = append(actions, Text(id+"/error", "Failed").Role("alert"))
	}
	if remove != nil {
		actions = append(actions, Button(id+"/remove", "Remove", remove).Variant(Ghost))
	}
	return Item(id, name, description, actions...)
}
func Bubble(id, text string) *Element {
	e := Card(id, paragraph(id+"/text", text)).Padding(12).Role("group").Variant(Secondary)
	e.flat = true
	return e
}
func Message(id, author string, end bool, children ...*Element) *Element {
	body := Column(id+"/body", children...).Grow(4)
	content := []*Element{body, Column(id + "/space").Grow(1)}
	if end {
		content[0], content[1] = content[1], content[0]
	}
	header := Text(id+"/author", author).Role("note")
	if end {
		header = Row(id+"/header", Column(id+"/header-space").Grow(1), header)
	}
	header.Hidden(author == "")
	return Column(id, header, Row(id+"/row", content...)).Role("article").Label(author)
}
func Marker(id, label string) *Element {
	e := Row(id, Separator(id+"/before").Grow(1), Text(id+"/text", label).Role("note"), Separator(id+"/after").Grow(1)).Role("separator").Label(label)
	e.centerItems = true
	return e
}

// MessageScroller follows appended content only while already at the end.
func MessageScroller(id string, state *ScrollState, children ...*Element) *Element {
	e := ScrollArea(id, state, children...).Role("log")
	e.followTail = true
	return e
}
func Chat(id string, state *ScrollState, children ...*Element) *Element {
	if state == nil {
		panic("baseui: nil chat scroll state")
	}
	return Column(id, MessageScroller(id+"/messages", state, children...), Button(id+"/latest", "Jump to latest", func() { state.Y = state.maxY }).Variant(Outline).Hidden(state.Y >= state.maxY-1)).Role("region").Label("Chat")
}

type MessageFeedback string

const (
	FeedbackNone MessageFeedback = ""
	FeedbackGood MessageFeedback = "good"
	FeedbackBad  MessageFeedback = "bad"
)

type AIMessageOptions struct {
	Author     string
	Streaming  bool
	Feedback   *MessageFeedback
	OnFeedback func(MessageFeedback)
	OnCopy     func(string)
	OnRetry    func()
	OnEdit     func(string)
}

// AIMessage currently displays plain text, with explicit application callbacks.
// Markdown, selectable rich text and clipboard completion feedback are pending.
func AIMessage(id, role, text string, o AIMessageOptions) *Element {
	end := role == "user"
	actions := []*Element{}
	if o.OnCopy != nil {
		actions = append(actions, Button(id+"/copy", "Copy", func() { o.OnCopy(text) }).Variant(Ghost).Disabled(o.Streaming || text == ""))
	}
	if !end && o.Feedback != nil {
		for _, v := range []MessageFeedback{FeedbackGood, FeedbackBad} {
			v := v
			actions = append(actions, Toggle(id+"/"+string(v), string(v), *o.Feedback == v, func(on bool) {
				*o.Feedback = FeedbackNone
				if on {
					*o.Feedback = v
				}
				if o.OnFeedback != nil {
					o.OnFeedback(*o.Feedback)
				}
			}).Disabled(o.Streaming))
		}
	}
	if !end && o.OnRetry != nil {
		actions = append(actions, Button(id+"/retry", "Retry", o.OnRetry).Variant(Ghost).Disabled(o.Streaming))
	}
	if end && o.OnEdit != nil {
		actions = append(actions, Button(id+"/edit", "Edit", func() { o.OnEdit(text) }).Variant(Ghost).Disabled(o.Streaming))
	}
	body := paragraph(id+"/text", text)
	if end {
		body = Bubble(id+"/bubble", text)
	}
	return Message(id, o.Author, end, body, Text(id+"/streaming", "Responding...").Role("status").Hidden(!o.Streaming), Row(id+"/actions", actions...))
}

type Source struct{ ID, Title, URL, Description string }

func sourceURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil
}
func sourceCard(id string, s Source, open func(string)) *Element {
	title := s.Title
	if title == "" {
		if u, err := url.Parse(s.URL); err == nil && u.Host != "" {
			title = u.Host
		} else {
			title = s.URL
		}
	}
	return Item(id, title, s.Description, Button(id+"/open", "Open source", func() {
		if open != nil && sourceURL(s.URL) {
			open(s.URL)
		}
	}).Variant(Outline).Disabled(open == nil || !sourceURL(s.URL))).Description(s.URL)
}
func Sources(id string, sources []Source, state *DisclosureState, open func(string)) *Element {
	children := []*Element{}
	seen := map[string]bool{}
	for _, s := range sources {
		if s.URL == "" || seen[s.URL] {
			continue
		}
		seen[s.URL] = true
		children = append(children, sourceCard(id+"/source/"+strconv.Itoa(len(children)), s, open))
	}
	return disclosure(id, fmt.Sprintf("Sources (%d)", len(children)), state, children...)
}
func InlineCitation(id, label string, source Source, open func(string)) *Element {
	return Row(id, Button(id+"/anchor", label, nil).Variant(Ghost).Description(source.Title), HoverCard(id+"/preview", id+"/anchor", sourceCard(id+"/source", source, open)))
}

type ContextUsage struct {
	Used, Limit, Input, Output int64
	Cost                       string
}

func Context(id string, usage ContextUsage) *Element {
	label := "Context · unknown limit"
	fraction := 0.
	if usage.Limit > 0 {
		fraction = min(1, max(0, float64(usage.Used)/float64(usage.Limit)))
		label = fmt.Sprintf("Context %.0f%%", 100*fraction)
	}
	details := fmt.Sprintf("Used: %d\nInput: %d\nOutput: %d", max(int64(0), usage.Used), max(int64(0), usage.Input), max(int64(0), usage.Output))
	if usage.Limit > 0 {
		details += fmt.Sprintf("\nLimit: %d", usage.Limit)
	} else {
		details += "\nLimit: unknown"
	}
	if strings.TrimSpace(usage.Cost) != "" {
		details += "\nCost: " + usage.Cost
	}
	return Row(id, Button(id+"/anchor", label, nil).Variant(Outline), HoverCard(id+"/details", id+"/anchor", Progress(id+"/progress", "Context used", fraction).Hidden(usage.Limit <= 0), Text(id+"/usage", details)))
}
