package phone

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "07 local", in: "0712345678", want: "254712345678"},
		{name: "01 local", in: "0112345678", want: "254112345678"},
		{name: "+2547", in: "+254712345678", want: "254712345678"},
		{name: "+2541", in: "+254112345678", want: "254112345678"},
		{name: "2547", in: "254712345678", want: "254712345678"},
		{name: "2541", in: "254112345678", want: "254112345678"},
		{name: "spaces and dashes", in: "07 123-456 78", want: "254712345678"},
		{name: "invalid prefix", in: "0812345678", wantErr: true},
		{name: "too short", in: "071234567", wantErr: true},
		{name: "too long", in: "07123456789", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "letters", in: "07abc45678", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Normalize(%q) err=nil, want error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize(%q) err=%v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("Normalize(%q)=%q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
