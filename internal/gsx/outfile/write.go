package outfile

import "os"

// WriteGeneratedFile writes src to outPath, always overwriting any existing file.
func WriteGeneratedFile(outPath string, src []byte) error {
	return os.WriteFile(outPath, src, 0o644)
}

