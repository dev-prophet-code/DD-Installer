package ui

import (
	"os"

	"fyne.io/fyne/v2"
)

const prefEffects = "effects"

// effectsDefault decides whether the animations start enabled: the user's
// saved choice, unless DD_INSTALLER_NO_EFFECTS is set.
func effectsDefault(a fyne.App) bool {
	if os.Getenv("DD_INSTALLER_NO_EFFECTS") != "" {
		return false
	}
	return a.Preferences().BoolWithFallback(prefEffects, true)
}

// setEffects turns every animation (sand, glow, glint, page veil) on or off
// and remembers the choice. Off means no timers run at all (battery friendly).
func (z *wizard) setEffects(on bool) {
	z.effectsOn = on
	z.a.Preferences().SetBool(prefEffects, on)
	if z.mote != nil {
		z.mote.setOn(on)
	}
	if z.glint != nil {
		z.glint.setOn(on)
	}
	if z.glow != nil {
		z.glow.setOn(on)
	}
	if z.fx != nil {
		z.fx.on = on
	}
	if z.effectsBtn != nil {
		if on {
			z.effectsBtn.SetText("Effects: on")
		} else {
			z.effectsBtn.SetText("Effects: off")
		}
	}
}
