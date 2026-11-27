package botTelegram

import "testing"

func TestParseProductCallbackID(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		wantID   int
		wantErr  bool
	}{
		{name: "valid product ID", callback: "add_123", wantID: 123},
		{name: "old callback with client supplied price", callback: "add_Товар 1 кг_1_100", wantErr: true},
		{name: "tampered price suffix", callback: "add_123_1_1", wantErr: true},
		{name: "zero ID", callback: "add_0", wantErr: true},
		{name: "negative ID", callback: "add_-5", wantErr: true},
		{name: "not a number", callback: "add_price", wantErr: true},
		{name: "wrong action", callback: "delete_123", wantErr: true},
		{name: "empty callback", callback: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProductCallbackID(tt.callback)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseProductCallbackID(%q) error = %v, wantErr %v", tt.callback, err, tt.wantErr)
			}
			if err == nil && got != tt.wantID {
				t.Fatalf("parseProductCallbackID(%q) = %d, want %d", tt.callback, got, tt.wantID)
			}
		})
	}
}
