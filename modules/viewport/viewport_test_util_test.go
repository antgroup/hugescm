package viewport

import (
	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

type object struct {
	item item.Item
}

func (i object) GetItem() item.Item {
	return i.item
}

func objectsEqual(a, b object) bool {
	if a.item == nil || b.item == nil {
		return a.item == b.item
	}
	return a.item.Content() == b.item.Content()
}

var _ Object = object{}

var (
	downKeyMsg       = MakeKeyMsg('j')
	halfPgDownKeyMsg = MakeKeyMsg('d')
	fullPgDownKeyMsg = MakeKeyMsg('f')
	upKeyMsg         = MakeKeyMsg('k')
	halfPgUpKeyMsg   = MakeKeyMsg('u')
	fullPgUpKeyMsg   = MakeKeyMsg('b')
	goToTopKeyMsg    = MakeKeyMsg('g')
	goToBottomKeyMsg = MakeKeyMsg('G')
	selectionStyle   = BlueFg
)

func newViewport(width, height int, options ...Option[object]) *Model[object] {
	styles := Styles{
		FooterStyle:       lipgloss.NewStyle(),
		SelectedItemStyle: selectionStyle,
	}

	options = append([]Option[object]{
		WithKeyMap[object](DefaultKeyMap()),
		WithStyles[object](styles),
	}, options...)

	return New(width, height, options...)
}
