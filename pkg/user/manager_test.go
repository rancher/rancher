package user_test

import (
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	usertypes "github.com/rancher/rancher/pkg/user/types"
	"github.com/stretchr/testify/require"
)

func TestManagerAlias(t *testing.T) {
	require.Equal(t, reflect.TypeFor[usertypes.Manager](), reflect.TypeFor[user.Manager]())
	for _, contextType := range []reflect.Type{reflect.TypeFor[config.ManagementContext](), reflect.TypeFor[config.ScaledContext]()} {
		field, ok := contextType.FieldByName("UserManager")
		require.True(t, ok)
		require.Equal(t, reflect.TypeFor[user.Manager](), field.Type)
	}
}
