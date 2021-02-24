package slowql

import "testing"

func TestQuery_Fingerprint(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    string
		wantErr bool
	}{
		{name: "pass", query: "foobar", want: "3858f62230ac3c915f300c664312c63f", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Query{
				Query: tt.query,
			}
			got, err := q.Fingerprint()
			if (err != nil) != tt.wantErr {
				t.Errorf("Query.Fingerprint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Query.Fingerprint() = %v, want %v", got, tt.want)
			}
		})
	}
}
