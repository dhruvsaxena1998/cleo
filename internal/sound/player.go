package sound

import (
	"os/exec"
	"runtime"
	"strconv"
)

type Player struct {
	bin    string
	args   func(file string) []string
	volume float64
}

func NewPlayer(volume float64) *Player {
	p := &Player{volume: volume}
	if runtime.GOOS == "darwin" {
		if path, err := exec.LookPath("afplay"); err == nil {
			p.bin = path
			vol := volume
			p.args = func(file string) []string {
				return []string{"-v", floatStr(vol), file}
			}
			return p
		}
	}
	for _, name := range []string{"paplay", "aplay", "play"} {
		if path, err := exec.LookPath(name); err == nil {
			p.bin = path
			p.args = func(file string) []string { return []string{file} }
			return p
		}
	}
	return p
}

func (p *Player) Available() bool { return p.bin != "" }

// Play is fire-and-forget: returns as soon as the child process is started.
// Errors are intentionally swallowed; sound failures must not block the agent.
func (p *Player) Play(file string) error {
	if !p.Available() {
		return nil
	}
	cmd := exec.Command(p.bin, p.args(file)...)
	return cmd.Start() // intentionally not Wait()
}

func floatStr(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
