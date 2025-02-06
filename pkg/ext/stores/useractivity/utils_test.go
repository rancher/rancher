package useractivity

import (
	"testing"
)

func Test_getUserActivityName(t *testing.T) {
	type args struct {
		uaName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name: "valid useractivity name",
			args: args{
				uaName: "ua_user-j9zn4_token-nkwxg",
			},
			want:    "user-j9zn4",
			want1:   "token-nkwxg",
			wantErr: false,
		},
		{
			name: "wrong useractivity name",
			args: args{
				uaName: "uauser-j9zn4_token-nkwxg",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := getUserActivityName(tt.args.uaName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getUserActivityName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getUserActivityName() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getUserActivityName() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_setUserActivityName(t *testing.T) {
	type args struct {
		user  string
		token string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "",
			args: args{
				user:  "user-abc",
				token: "token-abc",
			},
			want:    "ua_user-abc_token-abc",
			wantErr: false,
		},
		{
			name: "",
			args: args{
				user:  "",
				token: "token-abc",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "",
			args: args{
				user:  "user-abc",
				token: "",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "",
			args: args{
				user:  "",
				token: "",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := setUserActivityName(tt.args.user, tt.args.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("setUserActivityName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("setUserActivityName() = %v, want %v", got, tt.want)
			}
		})
	}
}
