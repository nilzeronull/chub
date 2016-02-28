// alsa output driver implementation.
package alsa

import "github.com/vchimishuk/chub/alsa/asoundlib"

// DriverName is the string name of the alsa driver.
var DriverName string = "alsa"

// Alsa aoutput driter handler structure.
type Alsa struct {
	handle *asoundlib.Handle
}

// New returns newly initialized alsa output driver.
func New() *Alsa {
	return &Alsa{}
}

func (a *Alsa) Open() error {
	a.handle = asoundlib.New()
	err := a.handle.Open("default", asoundlib.StreamTypePlayback, asoundlib.ModeBlock)
	if err != nil {
		return err
	}

	a.handle.SampleFormat = asoundlib.SampleFormatS16LE // XXX:

	return nil
}

func (a *Alsa) SetSampleRate(rate int) {
	a.handle.SampleRate = rate
	a.handle.ApplyHwParams()
}

func (a *Alsa) SetChannels(channels int) {
	a.handle.Channels = channels
	a.handle.ApplyHwParams()
}

func (a *Alsa) Wait(maxDelay int) (ok bool, err error) {
	return a.handle.Wait(maxDelay)
}

func (a *Alsa) AvailUpdate() (size int, err error) {
	return a.handle.AvailUpdate()
}

func (a *Alsa) Write(buf []byte) (written int, err error) {
	return a.handle.Write(buf)
}

func (a *Alsa) Pause() {
	a.handle.Pause()
}

func (a *Alsa) Paused() bool {
	return a.handle.Paused()
}

func (a *Alsa) Close() {
	a.handle.Close()
}