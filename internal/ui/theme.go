package ui

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Fonts: Rajdhani (body) and Cinzel Decorative (headings), both SIL OFL 1.1.
// The licence texts are shipped next to the font files in assets/fonts.
var (
	//go:embed assets/fonts/Rajdhani-Regular.ttf
	rajdhaniRegular []byte
	//go:embed assets/fonts/Rajdhani-Bold.ttf
	rajdhaniBold []byte
	//go:embed assets/fonts/CinzelDecorative-Bold.ttf
	cinzelBold []byte

	//go:embed assets/logo.png
	logoPNG []byte

	logoRes = fyne.NewStaticResource("dd-installer.png", logoPNG)

	fontBody    = fyne.NewStaticResource("Rajdhani-Regular.ttf", rajdhaniRegular)
	fontBodyB   = fyne.NewStaticResource("Rajdhani-Bold.ttf", rajdhaniBold)
	fontHeading = fyne.NewStaticResource("CinzelDecorative-Bold.ttf", cinzelBold)
)

// Arrakis palette. Original colours inspired by the desert, the spice and the
// "blue within blue" of the Fremen; no game assets are used.
var (
	colVoid     = color.NRGBA{R: 0x0b, G: 0x08, B: 0x06, A: 0xff} // deep night
	colPanel    = color.NRGBA{R: 0x16, G: 0x0f, B: 0x0a, A: 0x9c} // translucent sandstone
	colSand     = color.NRGBA{R: 0xec, G: 0xdc, B: 0xb8, A: 0xff} // text
	colSandDim  = color.NRGBA{R: 0xa8, G: 0x95, B: 0x72, A: 0xff} // secondary text
	colSpice    = color.NRGBA{R: 0xe0, G: 0x7b, B: 0x1a, A: 0xff} // melange orange
	colGold     = color.NRGBA{R: 0xf3, G: 0xb9, B: 0x4d, A: 0xff} // highlights
	colFremen   = color.NRGBA{R: 0x3f, G: 0x9b, B: 0xd1, A: 0xff} // blue within blue
	colDanger   = color.NRGBA{R: 0xd9, G: 0x4a, B: 0x38, A: 0xff}
	colOK       = color.NRGBA{R: 0x5f, G: 0xc0, B: 0x9a, A: 0xff}
	colWarn     = color.NRGBA{R: 0xf0, G: 0xc0, B: 0x40, A: 0xff}
	colLine     = color.NRGBA{R: 0xe0, G: 0x7b, B: 0x1a, A: 0x55}
	colInput    = color.NRGBA{R: 0x0c, G: 0x09, B: 0x06, A: 0xc8}
	colButton   = color.NRGBA{R: 0x3a, G: 0x27, B: 0x15, A: 0xd0}
	colHover    = color.NRGBA{R: 0xe0, G: 0x7b, B: 0x1a, A: 0x38}
	colDisabled = color.NRGBA{R: 0x7a, G: 0x6c, B: 0x55, A: 0xff}
)

type duneTheme struct{}

var _ fyne.Theme = duneTheme{}

func (duneTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return colVoid
	case theme.ColorNameForeground:
		return colSand
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameSelection:
		if n == theme.ColorNameSelection {
			return color.NRGBA{R: 0xe0, G: 0x7b, B: 0x1a, A: 0x66}
		}
		return colSpice
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0x1a, G: 0x0f, B: 0x05, A: 0xff}
	case theme.ColorNameButton:
		return colButton
	case theme.ColorNameInputBackground:
		return colInput
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return colLine
	case theme.ColorNameHover:
		return colHover
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0xf3, G: 0xb9, B: 0x4d, A: 0x55}
	case theme.ColorNameDisabled, theme.ColorNameDisabledButton:
		if n == theme.ColorNameDisabledButton {
			return color.NRGBA{R: 0x22, G: 0x18, B: 0x10, A: 0xa0}
		}
		return colDisabled
	case theme.ColorNamePlaceHolder:
		return colSandDim
	case theme.ColorNameError:
		return colDanger
	case theme.ColorNameSuccess:
		return colOK
	case theme.ColorNameWarning:
		return colWarn
	case theme.ColorNameHyperlink:
		return colFremen
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x14, G: 0x0d, B: 0x08, A: 0xf4}
	case theme.ColorNameShadow:
		return color.NRGBA{A: 0x60}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0xe0, G: 0x7b, B: 0x1a, A: 0x80}
	case theme.ColorNameHeaderBackground:
		return colPanel
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

func (duneTheme) Font(s fyne.TextStyle) fyne.Resource {
	switch {
	case s.Monospace: // repurposed: headings (we never need a monospace face)
		return fontHeading
	case s.Bold:
		return fontBodyB
	}
	return fontBody
}

func (duneTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (duneTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 16
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 19
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 3
	case theme.SizeNameInputBorder:
		return 1
	}
	return theme.DefaultTheme().Size(n)
}
