package cache

import (
	"strings"
	"testing"
)

func TestGenerateContentHash(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "empty content",
			content: "",
			want:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr: false,
		},
		{
			name:    "simple content",
			content: "hello world",
			want:    "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			got, err := GenerateContentHash(reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateContentHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GenerateContentHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateKeyFromMetadata(t *testing.T) {
	tests := []struct {
		name      string
		osType    string
		osVersion string
		filename  string
	}{
		{
			name:      "linux metadata",
			osType:    "linux",
			osVersion: "5.10.0",
			filename:  "test.bin",
		},
		{
			name:      "empty metadata",
			osType:    "",
			osVersion: "",
			filename:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateKeyFromMetadata(tt.osType, tt.osVersion, tt.filename)
			if len(got) != 32 {
				t.Errorf("GenerateKeyFromMetadata() returned key length = %v, want 32", len(got))
			}

			// Same inputs should generate same key
			got2 := GenerateKeyFromMetadata(tt.osType, tt.osVersion, tt.filename)
			if got != got2 {
				t.Errorf("GenerateKeyFromMetadata() not deterministic: first = %v, second = %v", got, got2)
			}
		})
	}
}
