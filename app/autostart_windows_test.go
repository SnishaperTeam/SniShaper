//go:build windows

package app

import "testing"

func TestSplitCommandLine(t *testing.T) {
	cases := []struct {
		in          string
		exe, args   string
		wantErr     bool
	}{
		{in: `"C:\Program Files\SniShaper\SniShaper.exe" --startup --autoproxy`, exe: `C:\Program Files\SniShaper\SniShaper.exe`, args: `--startup --autoproxy`},
		{in: `"C:\My App\app.exe" --startup`, exe: `C:\My App\app.exe`, args: `--startup`},
		{in: `C:\plain\app.exe`, exe: `C:\plain\app.exe`},
		{in: `  "C:\Spaced Path\app.exe"  `, exe: `C:\Spaced Path\app.exe`},
		{in: `   `, wantErr: true},
		{in: `"C:\unbalanced\app.exe --startup`, wantErr: true},
	}
	for _, c := range cases {
		exe, args, err := splitCommandLine(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("splitCommandLine(%q): expected error, got exe=%q args=%q", c.in, exe, args)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitCommandLine(%q): unexpected error: %v", c.in, err)
			continue
		}
		if exe != c.exe || args != c.args {
			t.Errorf("splitCommandLine(%q) = (%q, %q), want (%q, %q)", c.in, exe, args, c.exe, c.args)
		}
	}
}
