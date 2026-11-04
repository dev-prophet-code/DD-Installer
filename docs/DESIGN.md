# Design

The installer is styled to feel at home in the world of **Dune: Awakening**: Arrakis at night, spice orange and gold on deep sandstone, the "blue within blue" of the Fremen as accent. It is the same on Windows, macOS and Linux because it is drawn by the toolkit itself.

> The look is **original artwork inspired by the setting**. No game assets, logos or screenshots are used; this tool is unofficial and not affiliated with Funcom.

## Elements

| Element | Implementation |
|---|---|
| **Palette** | `internal/ui/theme.go`: night `#0b0806`, sandstone panels (translucent), sand text `#ecdcb8`, spice `#e07b1a`, gold `#f3b94d`, Fremen blue `#3f9bd1` for links |
| **Typography** | *Cinzel Decorative* for titles, *Rajdhani* (regular/bold) for text. Embedded; SIL OFL 1.1 (licences in `internal/ui/assets/fonts/`) |
| **Backdrop** | Generated at start (`art.go`): gradient night sky, stars, two moons, four layered dune ridges, grain and vignette. Deterministic, no image files |
| **Sand** | 46 drifting motes (`effects.go`) with twinkle; they speed up into a **sandstorm** while an installation runs |
| **Glow** | Title colour breathes between spice and gold; a light glint travels along the header line |
| **Transitions** | A dark veil fades away on every page change |
| **Panels** | Translucent sandstone cards with a thin spice edge so the dunes stay visible |
| **Icon** | Twin moons over a dune, drawn in code (`appIcon`) |
| **Cross-platform** | Fyne draws everything itself, so fonts, colours and effects are identical on all three systems |

## Changing the look

* Colours and sizes: `internal/ui/theme.go`.
* Backdrop: `renderBackdrop` in `art.go` (sky colours, ridge heights, moons).
* Effects: speed/amount of sand in `newMotes(n)` and `motes.setStorm`, glow timing in `effects.go`.
* Preview without a display: `DD_SHOTS=/some/dir go test ./internal/ui -run TestShots` writes PNGs of every page and dialog.

## Credits

* Rajdhani - Indian Type Foundry, SIL Open Font License 1.1
* Cinzel Decorative - Natanael Gama, SIL Open Font License 1.1
* UI toolkit: [Fyne](https://fyne.io) (BSD-3-Clause)
