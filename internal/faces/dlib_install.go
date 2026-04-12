package faces

import (
	"fmt"
	"os/exec"
	"runtime"
)

// DlibInstalled reports whether dlib is available on the system.
// It checks via pkg-config first, then falls back to a brew list check on macOS.
func DlibInstalled() bool {
	if exec.Command("pkg-config", "--exists", "dlib-1").Run() == nil {
		return true
	}
	// pkg-config may be absent on macOS; check brew as a fallback.
	if runtime.GOOS == "darwin" {
		return exec.Command("brew", "--prefix", "dlib").Run() == nil
	}
	return false
}

// EnsureDlib checks whether dlib is installed and installs it if not.
// progress is called with status messages suitable for display in a spinner.
// Returns an error if installation is unsupported or fails.
func EnsureDlib(progress func(string)) error {
	if DlibInstalled() {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		return installDlibBrew(progress)
	case "linux":
		return installDlibApt(progress)
	default:
		return fmt.Errorf(
			"automatic dlib installation is not supported on %s — install manually: https://github.com/Kagami/go-face#requirements",
			runtime.GOOS,
		)
	}
}

func installDlibBrew(progress func(string)) error {
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("homebrew not found; run: brew install dlib")
	}
	progress("Installing dlib via Homebrew (this may take a few minutes)...")
	out, err := exec.Command("brew", "install", "dlib").CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew install dlib: %w\n%s", err, out)
	}
	return nil
}

func installDlibApt(progress func(string)) error {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("apt-get not found; install dlib manually: https://github.com/Kagami/go-face#requirements")
	}
	progress("Installing dlib via apt-get (this may take a few minutes)...")
	out, err := exec.Command(
		"sudo", "apt-get", "install", "-y",
		"libdlib-dev", "libblas-dev", "liblapack-dev", "libopenblas-dev",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get install dlib: %w\n%s", err, out)
	}
	return nil
}
