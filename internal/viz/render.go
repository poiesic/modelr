package viz

import (
	"bytes"
	"fmt"
	"os/exec"
)

// GraphvizAvailable checks if the `dot` command is in PATH.
func GraphvizAvailable() bool {
	_, err := exec.LookPath("dot")
	return err == nil
}

// RenderDOT pipes DOT source through graphviz to produce SVG or PNG output.
// Format must be "svg" or "png".
func RenderDOT(dot string, format string) ([]byte, error) {
	if !GraphvizAvailable() {
		return nil, fmt.Errorf("graphviz is not installed; install it to render %s (e.g., apt install graphviz)", format)
	}

	cmd := exec.Command("dot", "-T"+format)
	cmd.Stdin = bytes.NewBufferString(dot)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("graphviz error: %s: %w", stderr.String(), err)
	}

	return stdout.Bytes(), nil
}
