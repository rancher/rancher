package vsphere

import (
	"testing"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/stretchr/testify/assert"
)

func Test_checkGuestId(t *testing.T) {
	type args struct {
		g string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "guestId-windows",
			args: args{
				g: rke2.WindowsMachineOS,
			},
			want:    rke2.WindowsMachineOS,
			wantErr: false,
		},
		{
			name: "guestId-linux",
			args: args{
				g: rke2.DefaultMachineOS,
			},
			want:    rke2.DefaultMachineOS,
			wantErr: false,
		},
		{
			// test that the guestId is defaulted to linux if g is
			// anything other than "windows"
			name: "guestId-empty",
			args: args{
				g: "",
			},
			want:    rke2.DefaultMachineOS,
			wantErr: false,
		},
		{
			name: "guestId-unsupported",
			args: args{
				g: "darwin",
			},
			want:    "darwin",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)

			// act
			guestId := checkGuestID(tt.args.g)

			// assert
			a.NotEmpty(guestId)
			if tt.wantErr {
				a.NotEqual(tt.want, guestId)
			} else {
				a.Equal(tt.want, guestId)
			}
		})
	}
}
