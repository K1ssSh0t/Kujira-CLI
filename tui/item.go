package tui

import "fmt"

type Item struct {
	TitleValue       string
	DescriptionValue string
}

func (i Item) Title() string       { return i.TitleValue }
func (i Item) Description() string { return i.DescriptionValue }
func (i Item) FilterValue() string { return i.TitleValue }

func (i Item) String() string {
	return fmt.Sprintf("%s: %s", i.TitleValue, i.DescriptionValue)
}
