package k6env

import "testing"

func TestParseCloudLoginStatus(t *testing.T) {
	t.Parallel()

	// notSetOutput mirrors `k6 cloud login --show` for a logged-out user on
	// k6 v2.x: a banner followed by placeholder fields.
	const notSetOutput = `
         /\      Grafana   /‾‾/
    /\  /  \     |\  __   /  /
   /  \/    \    | |/ /  /   ‾‾\

  token: <not set>
  stack-id: <not set>
  stack-url: <not set>
  default-project-id: <not set>
`

	const loggedInOutput = `
  token: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
  stack-id: 12345
`

	tests := []struct {
		name       string
		output     string
		wantLogged bool
		wantErr    bool
	}{
		{name: "logged out shows not set", output: notSetOutput, wantLogged: false},
		{name: "logged in shows token", output: loggedInOutput, wantLogged: true},
		{name: "token value present but non-hex is still logged in", output: "  token: abc-masked-value", wantLogged: true},
		{name: "empty token value is logged out", output: "  token: ", wantLogged: false},
		{name: "no token line is an error", output: "  stack-id: <not set>", wantErr: true},
		{name: "empty output is an error", output: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logged, err := parseCloudLoginStatus(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseCloudLoginStatus(%q) = (%v, nil), want error", tt.output, logged)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCloudLoginStatus(%q) returned unexpected error: %v", tt.output, err)
			}
			if logged != tt.wantLogged {
				t.Fatalf("parseCloudLoginStatus(%q) = %v, want %v", tt.output, logged, tt.wantLogged)
			}
		})
	}
}
