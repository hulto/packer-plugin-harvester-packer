// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"testing"
)

// TestSpecialKeys verifies that all special key sequences are defined.
func TestSpecialKeys(t *testing.T) {
	requiredKeys := []string{
		"enter", "return", "bs", "backspace", "del", "delete",
		"tab", "esc", "escape", "up", "down", "left", "right",
		"home", "end", "pageup", "pagedown", "insert", "spacebar",
		"f1", "f2", "f3", "f4", "f5", "f6",
		"f7", "f8", "f9", "f10", "f11", "f12",
		"leftshift", "rightshift", "leftctrl", "rightctrl",
		"leftalt", "rightalt", "leftsuper", "rightsuper",
	}
	for _, k := range requiredKeys {
		if _, ok := specialKeys[k]; !ok {
			t.Errorf("missing special key %q in specialKeys map", k)
		}
	}
}

// TestRuneToKeySym verifies basic character-to-keysym mapping.
func TestRuneToKeySym(t *testing.T) {
	tests := []struct {
		input      rune
		wantKeySym uint32
		wantShift  bool
	}{
		{'a', 0x61, false},
		{'z', 0x7a, false},
		{'A', 0x41, true},
		{'Z', 0x5a, true},
		{'0', 0x30, false},
		{'9', 0x39, false},
		{' ', 0x20, false},
	}
	for _, tt := range tests {
		keySym, shift := runeToKeySym(tt.input)
		if keySym != tt.wantKeySym {
			t.Errorf("runeToKeySym(%q) keySym = 0x%x, want 0x%x", tt.input, keySym, tt.wantKeySym)
		}
		if shift != tt.wantShift {
			t.Errorf("runeToKeySym(%q) shift = %v, want %v", tt.input, shift, tt.wantShift)
		}
	}
}

// TestIsShiftSymbol verifies that shift symbols are correctly detected.
func TestIsShiftSymbol(t *testing.T) {
	shiftChars := []rune{'!', '@', '#', '$', '%', '^', '&', '*', '(', ')', '_', '+', '{', '}', '|', ':', '"', '<', '>', '?', '~'}
	for _, r := range shiftChars {
		if !isShiftSymbol(r) {
			t.Errorf("isShiftSymbol(%q) = false, want true", r)
		}
	}
	nonShiftChars := []rune{'a', 'b', '1', '2', '.', ',', ';', '-', '='}
	for _, r := range nonShiftChars {
		if isShiftSymbol(r) {
			t.Errorf("isShiftSymbol(%q) = true, want false", r)
		}
	}
}
