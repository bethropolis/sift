package qr

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeFinderAndQuietZone(t *testing.T) {
	code, err := Encode("https://example.com/#pair=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", M)
	if err != nil {
		t.Fatal(err)
	}
	if code.Size < 21 || code.Size%4 != 1 {
		t.Fatalf("size = %d, want 4v+17", code.Size)
	}
	// Three finder patterns: top-left, top-right, bottom-left.
	for _, origin := range [][2]int{{0, 0}, {code.Size - 7, 0}, {0, code.Size - 7}} {
		for y := 0; y < 7; y++ {
			for x := 0; x < 7; x++ {
				want := x == 0 || x == 6 || y == 0 || y == 6 || (x >= 2 && x <= 4 && y >= 2 && y <= 4)
				if got := code.Black(origin[0]+x, origin[1]+y); got != want {
					t.Fatalf("finder at %v (%d,%d) = %v, want %v", origin, x, y, got, want)
				}
			}
		}
	}
	text := code.Terminal()
	if !strings.ContainsAny(text, "█▀▄") {
		t.Fatalf("terminal output has no half-block modules:\n%s", text)
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	wantLines := (code.Size + 4 + 1) / 2
	if len(lines) != wantLines {
		t.Fatalf("terminal lines = %d, want %d", len(lines), wantLines)
	}
}

// TestDecodeWithZbar verifies the matrix with a real decoder when one is
// installed. CI without zbarimg skips; the finder-pattern test still runs.
func TestDecodeWithZbar(t *testing.T) {
	zbar, err := exec.LookPath("zbarimg")
	if err != nil {
		t.Skip("zbarimg not installed")
	}
	text := "https://example.com/#pair=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	code, err := Encode(text, M)
	if err != nil {
		t.Fatal(err)
	}
	const quiet = 4
	const scale = 8
	side := (code.Size + quiet*2) * scale
	img := image.NewGray(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			if code.Black(x/scale-quiet, y/scale-quiet) {
				img.SetGray(x, y, color.Gray{Y: 0})
			} else {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pair.png")
	if err := os.WriteFile(path, pngData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(zbar, "--quiet", "--raw", path).CombinedOutput()
	if err != nil {
		t.Fatalf("zbarimg: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != text {
		t.Fatalf("decoded %q, want %q", got, text)
	}
}
