package serviceaccounttoken

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestQueueDequeue(t *testing.T) {
	names := []*corev1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "item-1", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-2", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-3", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-4", Namespace: "default"}},
	}
	q := newQueue[*corev1.Secret]()
	require.Equal(t, 0, q.List.Len())

	q.enqueue(names...)

	require.Equal(t, 4, q.List.Len())

	elements1 := q.dequeue(2)
	require.Equal(t, names[0:2], elements1)

	elements2 := q.dequeue(2)
	require.Equal(t, names[2:4], elements2)

	elements3 := q.dequeue(2)
	require.Nil(t, elements3)

	require.Equal(t, 0, q.List.Len())
}

func TestQueueEnqueueDequeue(t *testing.T) {
	names := []*corev1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "item-1", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-2", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-3", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-4", Namespace: "default"}},
	}
	q := newQueue[*corev1.Secret](names...)

	elements1 := q.dequeue(2)
	require.Equal(t, names[0:2], elements1)

	elements2 := q.dequeue(2)
	require.Equal(t, names[2:4], elements2)

	elements3 := q.dequeue(2)
	require.Nil(t, elements3)
}
