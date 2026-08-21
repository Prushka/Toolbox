// Package hid controls the Toolbox Arduino Leonardo HID firmware.
//
// The Leonardo sends input through USB HID, so Windows treats events exactly
// like input from a physical keyboard and mouse. Commands and acknowledgements
// use the board's separate USB CDC serial interface.
package hid
