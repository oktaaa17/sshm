package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type themeStyles struct {
	HeaderTitle    lipgloss.Style
	HeaderStats    lipgloss.Style
	HeaderMeta     lipgloss.Style
	Divider        lipgloss.Style
	SearchPrompt   lipgloss.Style
	SearchText     lipgloss.Style
	TabActive      lipgloss.Style
	TabInactive    lipgloss.Style
	GroupHeader    lipgloss.Style
	HostSelected   lipgloss.Style
	HostNormal     lipgloss.Style
	HostDetail     lipgloss.Style
	BadgePW        lipgloss.Style
	BadgeKey       lipgloss.Style
	BadgeJump      lipgloss.Style
	TagBadge       lipgloss.Style
	ForwardBadge   lipgloss.Style
	ProbeOnline    lipgloss.Style
	ProbeOffline   lipgloss.Style
	ProbeChecking  lipgloss.Style
	InspectorBox   lipgloss.Style
	InspectorTitle lipgloss.Style
	InspectorKey   lipgloss.Style
	InspectorVal   lipgloss.Style
	Dim            lipgloss.Style
	Footer         lipgloss.Style
	Watermark      lipgloss.Style
	FormTitle      lipgloss.Style
	FormLabel      lipgloss.Style
	FormActive     lipgloss.Style
	FormBox        lipgloss.Style
	ErrorText      lipgloss.Style
	StatusText     lipgloss.Style
	PinStar        lipgloss.Style
}

var themeOrder = []string{"minimal-dark", "minimal-light", "mono"}

func nextThemeName(current string) string {
	cur := strings.ToLower(strings.TrimSpace(current))
	for i, name := range themeOrder {
		if name == cur {
			return themeOrder[(i+1)%len(themeOrder)]
		}
	}
	return themeOrder[0]
}

func newTheme(name string) themeStyles {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mono":
		return themeStyles{
			HeaderTitle:    lipgloss.NewStyle().Bold(true),
			HeaderStats:    lipgloss.NewStyle().Faint(true),
			HeaderMeta:     lipgloss.NewStyle().Faint(true),
			Divider:        lipgloss.NewStyle().Faint(true),
			SearchPrompt:   lipgloss.NewStyle().Bold(true),
			SearchText:     lipgloss.NewStyle(),
			TabActive:      lipgloss.NewStyle().Bold(true).Underline(true),
			TabInactive:    lipgloss.NewStyle().Faint(true),
			GroupHeader:    lipgloss.NewStyle().Bold(true),
			HostSelected:   lipgloss.NewStyle().Reverse(true).Bold(true),
			HostNormal:     lipgloss.NewStyle(),
			HostDetail:     lipgloss.NewStyle().Faint(true),
			BadgePW:        lipgloss.NewStyle().Faint(true),
			BadgeKey:       lipgloss.NewStyle().Faint(true),
			BadgeJump:      lipgloss.NewStyle().Faint(true),
			TagBadge:       lipgloss.NewStyle().Faint(true),
			ForwardBadge:   lipgloss.NewStyle().Faint(true),
			ProbeOnline:    lipgloss.NewStyle().Bold(true),
			ProbeOffline:   lipgloss.NewStyle().Faint(true),
			ProbeChecking:  lipgloss.NewStyle().Faint(true),
			InspectorBox:   lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).Padding(0, 1, 0, 2),
			InspectorTitle: lipgloss.NewStyle().Bold(true).Underline(true),
			InspectorKey:   lipgloss.NewStyle().Faint(true),
			InspectorVal:   lipgloss.NewStyle(),
			Dim:            lipgloss.NewStyle().Faint(true),
			Footer:         lipgloss.NewStyle().Faint(true),
			Watermark:      lipgloss.NewStyle().Faint(true).Italic(true),
			FormTitle:      lipgloss.NewStyle().Bold(true),
			FormLabel:      lipgloss.NewStyle().Faint(true),
			FormActive:     lipgloss.NewStyle().Bold(true).Underline(true),
			FormBox:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2),
			ErrorText:      lipgloss.NewStyle().Bold(true),
			StatusText:     lipgloss.NewStyle().Faint(true),
			PinStar:        lipgloss.NewStyle().Bold(true),
		}
	case "minimal-light":
		return themeStyles{
			HeaderTitle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("24")),
			HeaderStats:    lipgloss.NewStyle().Foreground(lipgloss.Color("60")),
			HeaderMeta:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
			Divider:        lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
			SearchPrompt:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("24")),
			SearchText:     lipgloss.NewStyle().Foreground(lipgloss.Color("235")),
			TabActive:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("24")).Padding(0, 1),
			TabInactive:    lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Padding(0, 1),
			GroupHeader:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("236")),
			HostSelected:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("24")),
			HostNormal:     lipgloss.NewStyle().Foreground(lipgloss.Color("235")),
			HostDetail:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
			BadgePW:        lipgloss.NewStyle().Foreground(lipgloss.Color("130")),
			BadgeKey:       lipgloss.NewStyle().Foreground(lipgloss.Color("28")),
			BadgeJump:      lipgloss.NewStyle().Foreground(lipgloss.Color("61")),
			TagBadge:       lipgloss.NewStyle().Foreground(lipgloss.Color("25")),
			ForwardBadge:   lipgloss.NewStyle().Foreground(lipgloss.Color("91")),
			ProbeOnline:    lipgloss.NewStyle().Foreground(lipgloss.Color("28")),
			ProbeOffline:   lipgloss.NewStyle().Foreground(lipgloss.Color("160")),
			ProbeChecking:  lipgloss.NewStyle().Foreground(lipgloss.Color("136")),
			InspectorBox:   lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(lipgloss.Color("250")).Padding(0, 1, 0, 2),
			InspectorTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("24")),
			InspectorKey:   lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
			InspectorVal:   lipgloss.NewStyle().Foreground(lipgloss.Color("235")),
			Dim:            lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
			Footer:         lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
			Watermark:      lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true),
			FormTitle:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("24")),
			FormLabel:      lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
			FormActive:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("24")),
			FormBox:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("24")).Padding(1, 2),
			ErrorText:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("160")),
			StatusText:     lipgloss.NewStyle().Foreground(lipgloss.Color("28")),
			PinStar:        lipgloss.NewStyle().Foreground(lipgloss.Color("172")),
		}
	default: // minimal-dark
		return themeStyles{
			HeaderTitle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
			HeaderStats:    lipgloss.NewStyle().Foreground(lipgloss.Color("110")),
			HeaderMeta:     lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
			Divider:        lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
			SearchPrompt:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
			SearchText:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
			TabActive:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("81")).Padding(0, 1),
			TabInactive:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Padding(0, 1),
			GroupHeader:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250")),
			HostSelected:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("81")),
			HostNormal:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
			HostDetail:     lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
			BadgePW:        lipgloss.NewStyle().Foreground(lipgloss.Color("215")),
			BadgeKey:       lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
			BadgeJump:      lipgloss.NewStyle().Foreground(lipgloss.Color("141")),
			TagBadge:       lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
			ForwardBadge:   lipgloss.NewStyle().Foreground(lipgloss.Color("177")),
			ProbeOnline:    lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
			ProbeOffline:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
			ProbeChecking:  lipgloss.NewStyle().Foreground(lipgloss.Color("221")),
			InspectorBox:   lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(lipgloss.Color("238")).Padding(0, 1, 0, 2),
			InspectorTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
			InspectorKey:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
			InspectorVal:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
			Dim:            lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
			Footer:         lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
			Watermark:      lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true),
			FormTitle:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
			FormLabel:      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
			FormActive:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
			FormBox:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("81")).Padding(1, 2),
			ErrorText:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")),
			StatusText:     lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
			PinStar:        lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
		}
	}
}

func groupDot(colorName string, mono bool) string {
	if mono {
		return "●"
	}
	var c lipgloss.Color
	switch strings.ToLower(strings.TrimSpace(colorName)) {
	case "red":
		c = lipgloss.Color("203")
	case "yellow", "orange":
		c = lipgloss.Color("221")
	case "green":
		c = lipgloss.Color("114")
	case "blue":
		c = lipgloss.Color("75")
	case "cyan":
		c = lipgloss.Color("81")
	case "magenta", "purple":
		c = lipgloss.Color("177")
	default:
		c = lipgloss.Color("245")
	}
	return lipgloss.NewStyle().Foreground(c).Render("●")
}
