package main

import "testing"

func TestShimShadowsBinary(t *testing.T) {
	const ourBin = `C:\Users\cush\go\bin\store.exe`
	const shim = `C:\Users\cush\AppData\Local\Microsoft\WindowsApps\store.exe`

	tests := []struct {
		name      string
		whereOut  string
		exe       string
		want      bool
	}{
		{
			name:     "shim listed first, our binary second",
			whereOut: shim + "\n" + ourBin + "\n",
			exe:      ourBin,
			want:     true,
		},
		{
			name:     "our binary listed first",
			whereOut: ourBin + "\n" + shim + "\n",
			exe:      ourBin,
			want:     false,
		},
		{
			name:     "only our binary on PATH",
			whereOut: ourBin + "\n",
			exe:      ourBin,
			want:     false,
		},
		{
			name:     "only the shim on PATH",
			whereOut: shim + "\n",
			exe:      ourBin,
			want:     true,
		},
		{
			name:     "no matches",
			whereOut: "",
			exe:      ourBin,
			want:     false,
		},
		{
			name:     "case-insensitive match on exe path",
			whereOut: `c:\users\cush\go\bin\store.exe` + "\n",
			exe:      ourBin,
			want:     false,
		},
		{
			name:     "case-insensitive match on WindowsApps segment",
			whereOut: `C:\Users\cush\AppData\Local\Microsoft\WINDOWSAPPS\store.exe` + "\n",
			exe:      ourBin,
			want:     true,
		},
		{
			name:     "third-party store.exe somewhere else does not trigger hint",
			whereOut: `C:\tools\store.exe` + "\n" + ourBin + "\n",
			exe:      ourBin,
			want:     false,
		},
		{
			name:     "leading blank lines are skipped",
			whereOut: "\n\n" + shim + "\n",
			exe:      ourBin,
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shimShadowsBinary(tt.whereOut, tt.exe); got != tt.want {
				t.Fatalf("shimShadowsBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}
