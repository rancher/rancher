//go:build !windows

package clean

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	"github.com/rancher/wrangler/v3/pkg/generic"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

func TestCleanOrphans(t *testing.T) {
	t.Parallel()
	groupSubjects := []v1.Subject{
		{
			Kind: v1.GroupKind,
			Name: "Dogs",
		},
	}
	orphanBindingWithBadOldLabel := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orphan binding with nonexistent old-style PRTB label",
			Labels: map[string]string{
				"ns1-non-existent-uid": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	orphanBindingWithBadNewLabel := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orphan binding with nonexistent new-style PRTB label",
			Labels: map[string]string{
				"unknown_unknown": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	orphanBindingWithHashedLabel := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orphan binding with a new-style PRTB label that has a hash",
			Labels: map[string]string{
				"aaaaaaaaaa_whalewhalewhalewhalewhalewhalewhalewhalewhalew-4076a": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	orphanBindingWithStrangeLabel := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orphan binding with a new-style PRTB label that has a long namespace name",
			Labels: map[string]string{
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-8k9i4": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	nonOrphanBindingUnrelatedToPRTB := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-orphan binding that is unrelated to any project role template bindings",
			Labels: map[string]string{
				"default-8k9i4": "hello",
			},
		},
		Subjects: groupSubjects,
	}
	nonOrphanBindingWithStrangeLabel := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-orphan binding with a new-style PRTB label that has a long namespace name",
			Labels: map[string]string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-37a42": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	nonOrphanBindingWithSimpleLabel := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-orphan binding with hashed simple label",
			Labels: map[string]string{
				"default_foo": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	nonOrphanBindingWithHashedLabel := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-orphan binding with hashed label",
			Labels: map[string]string{
				"p-kmc2v_grouplonglonglonglonglonglonglonglonglonglonglong-a59f5": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	nonOrphanBindingWithUIDLabel := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-orphan binding with UID label",
			Labels: map[string]string{
				"c8bcebc8-779d-4fe7-a367-aa80897fc7b1": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	nonOrphanUserBinding := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-orphan user binding",
			Labels: map[string]string{
				"ns1-non-existent-prtb": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: []v1.Subject{
			{
				Kind: v1.UserKind,
				Name: "Bob",
			},
		},
	}
	allBindings := []v1.RoleBinding{
		orphanBindingWithBadOldLabel,
		orphanBindingWithBadNewLabel,
		orphanBindingWithStrangeLabel,
		orphanBindingWithHashedLabel,
		nonOrphanBindingUnrelatedToPRTB,
		nonOrphanBindingWithStrangeLabel,
		nonOrphanBindingWithSimpleLabel,
		nonOrphanBindingWithHashedLabel,
		nonOrphanBindingWithUIDLabel,
		nonOrphanUserBinding,
	}

	tests := []struct {
		name         string
		client       *orphanBindingsCleanup
		remainingRBs []v1.RoleBinding
		dryRun       bool
		wantErr      bool
	}{
		{
			name:         "dry run with no bindings deleted",
			dryRun:       true,
			client:       newStandardClient(allBindings),
			remainingRBs: allBindings,
		},
		{
			name:   "some bindings deleted",
			dryRun: false,
			client: newStandardClient(allBindings),
			remainingRBs: []v1.RoleBinding{
				nonOrphanBindingUnrelatedToPRTB,
				nonOrphanBindingWithStrangeLabel,
				nonOrphanBindingWithSimpleLabel,
				nonOrphanBindingWithHashedLabel,
				nonOrphanBindingWithUIDLabel,
				nonOrphanUserBinding,
			},
		},
		{
			name:    "failed to list project role template bindings",
			wantErr: true,
			client: &orphanBindingsCleanup{
				prtbs: &fakePRTBClient{
					bindings: []v3.ProjectRoleTemplateBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "bad",
							},
						},
					},
				},
				roleBindings: &fakeRoleBindingClient{
					bindings: allBindings,
				},
				prtbHashes: make(map[string]struct{}),
				prtbUIDs:   make(map[string]struct{}),
			},
		},
		{
			name:    "failed to list role bindings",
			wantErr: true,
			client: &orphanBindingsCleanup{
				prtbs: &fakePRTBClient{
					bindings: []v3.ProjectRoleTemplateBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "hello",
							},
						},
					},
				},
				roleBindings: &fakeRoleBindingClient{
					bindings: []v1.RoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "bad",
							},
						},
					},
				},
				prtbHashes: make(map[string]struct{}),
				prtbUIDs:   make(map[string]struct{}),
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := test.client.cleanOrphans(test.dryRun)
			if test.wantErr && err == nil {
				t.Error("expected an error, but didn't get it", err)
				return
			}
			if !test.wantErr && err != nil {
				t.Errorf("got an unexpected error: %v", err)
				return
			}
			if !test.wantErr {
				current, _ := test.client.roleBindings.List("", metav1.ListOptions{})
				if !reflect.DeepEqual(current.Items, test.remainingRBs) {
					t.Errorf("\nexpected:\n%s\ngot:\n%s",
						strings.Join(getRoleBindingsNames(test.remainingRBs), "\n"),
						strings.Join(getRoleBindingsNames(current.Items), "\n"))
				}
			}
		})
	}
}

func newStandardClient(roleBindings []v1.RoleBinding) *orphanBindingsCleanup {
	// Copy the role bindings to avoid unwanted concurrent modification. The slice is shared by multiple fake clients.
	rbs := make([]v1.RoleBinding, len(roleBindings))
	copy(rbs, roleBindings)
	return &orphanBindingsCleanup{
		prtbs: &fakePRTBClient{bindings: []v3.ProjectRoleTemplateBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
					UID:       "c8bcebc8-779d-4fe7-a367-aa80897fc7b1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grouplonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglong",
					Namespace: "p-kmc2v",
					UID:       "abc123",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "whale",
					Namespace: strings.Repeat("a", 63),
				},
			},
		}},
		prtbHashes: make(map[string]struct{}),
		prtbUIDs:   make(map[string]struct{}),
		roleBindings: &fakeRoleBindingClient{
			bindings: rbs,
		},
	}
}

func TestIsOrphanBinding(t *testing.T) {
	t.Parallel()
	groupSubjects := []v1.Subject{
		{
			Kind: v1.GroupKind,
			Name: "Dogs",
		},
	}
	client := orphanBindingsCleanup{
		prtbHashes: map[string]struct{}{
			"p-kmc2v_grouplonglonglonglonglonglonglonglonglonglonglong-a59f5": {},
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-4c6e1": {},
			"p-kmc2v_c-123xyz": {},
		},
		prtbUIDs: map[string]struct{}{
			"c8bcebc8-779d-4fe7-a367-aa80897fc7b1": {},
		},
	}

	tests := []struct {
		name     string
		binding  *v1.RoleBinding
		isOrphan bool
	}{
		{
			name:     "nil binding",
			binding:  nil,
			isOrphan: false,
		},
		{
			name:     "no subjects",
			binding:  &v1.RoleBinding{},
			isOrphan: false,
		},
		{
			name: "no group subjects",
			binding: &v1.RoleBinding{
				Subjects: []v1.Subject{
					{
						Kind: v1.UserKind,
						Name: "Pug",
					},
				},
			},
			isOrphan: false,
		},
		{
			name: "no matching new label or uid",
			binding: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"hello": "world",
					},
				},
				Subjects: groupSubjects,
			},
			isOrphan: false,
		},
		{
			name: "matching new label",
			binding: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"p-kmc2v_c-123xyz": auth.PrtbInClusterBindingOwner,
					},
				},
				Subjects: groupSubjects,
			},
			isOrphan: false,
		},
		{
			name: "matching new label with hash",
			binding: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"p-kmc2v_grouplonglonglonglonglonglonglonglonglonglonglong-a59f5": auth.PrtbInClusterBindingOwner,
					},
				},
				Subjects: groupSubjects,
			},
			isOrphan: false,
		},
		{
			name: "matching uid",
			binding: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"c8bcebc8-779d-4fe7-a367-aa80897fc7b1": auth.PrtbInClusterBindingOwner,
					},
				},
				Subjects: groupSubjects,
			},
			isOrphan: false,
		},
		{
			name: "matching new label but missing prtb",
			binding: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"ns1_missing1": auth.PrtbInClusterBindingOwner,
					},
				},
				Subjects: groupSubjects,
			},
			isOrphan: true,
		},
		{
			name: "matching uid but it is unknown",
			binding: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"uid-unknown": auth.PrtbInClusterBindingOwner,
					},
				},
				Subjects: groupSubjects,
			},
			isOrphan: true,
		},
		{
			name: "known binding with a very long namespace name",
			binding: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-4c6e1": auth.PrtbInClusterBindingOwner,
					},
				},
				Subjects: groupSubjects,
			},
			isOrphan: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			isOrphan := client.isOrphanBinding(test.binding)
			if isOrphan != test.isOrphan {
				t.Errorf("expected isOrphan to be %v, but got %v", test.isOrphan, isOrphan)
			}
		})
	}
}

type fakeRoleBindingClient struct {
	bindings []v1.RoleBinding
}

func (c *fakeRoleBindingClient) List(_ string, _ metav1.ListOptions) (*v1.RoleBindingList, error) {
	var lst v1.RoleBindingList
	// Build a new collection to prevent deletion of items during iteration later.
	for i := range c.bindings {
		if c.bindings[i].Name == "bad" {
			return nil, errors.New("failed to list role bindings")
		}
		lst.Items = append(lst.Items, c.bindings[i])
	}
	return &lst, nil
}

func (c *fakeRoleBindingClient) Delete(_, name string, _ *metav1.DeleteOptions) error {
	for i, v := range c.bindings {
		if v.Name == name {
			c.bindings = append(c.bindings[:i], c.bindings[i+1:]...)
			return nil
		}
	}
	return nil
}

func (c *fakeRoleBindingClient) Create(_ *v1.RoleBinding) (*v1.RoleBinding, error) {
	panic("implement me")
}

func (c *fakeRoleBindingClient) Update(_ *v1.RoleBinding) (*v1.RoleBinding, error) {
	panic("implement me")
}

func (c *fakeRoleBindingClient) UpdateStatus(_ *v1.RoleBinding) (*v1.RoleBinding, error) {
	panic("implement me")
}

func (c *fakeRoleBindingClient) Get(_, _ string, _ metav1.GetOptions) (*v1.RoleBinding, error) {
	panic("implement me")
}

func (c *fakeRoleBindingClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (c *fakeRoleBindingClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *v1.RoleBinding, err error) {
	panic("implement me")
}

func (c *fakeRoleBindingClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*v1.RoleBinding, *v1.RoleBindingList], error) {
	panic("implement me")
}

func (c *fakeRoleBindingClient) DeleteCollection(_ string, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	panic("implement me")
}

type fakePRTBClient struct {
	bindings []v3.ProjectRoleTemplateBinding
}

func (c *fakePRTBClient) List(_ string, _ metav1.ListOptions) (*v3.ProjectRoleTemplateBindingList, error) {
	var lst v3.ProjectRoleTemplateBindingList
	for _, b := range c.bindings {
		if b.Name == "bad" {
			return nil, errors.New("failed to list PRTBs")
		}
		lst.Items = append(lst.Items, b)
	}
	return &lst, nil
}

func (c *fakePRTBClient) Create(_ *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	panic("implement me")
}

func (c *fakePRTBClient) Update(_ *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	panic("implement me")
}

func (c *fakePRTBClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}

func (c *fakePRTBClient) Get(_, _ string, _ metav1.GetOptions) (*v3.ProjectRoleTemplateBinding, error) {
	panic("implement me")
}

func (c *fakePRTBClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (c *fakePRTBClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *v3.ProjectRoleTemplateBinding, err error) {
	panic("implement me")
}

func (c *fakePRTBClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList], error) {
	panic("implement me")
}

func (c *fakePRTBClient) UpdateStatus(_ *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	panic("implement me")
}

func (c *fakePRTBClient) DeleteCollection(_ string, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	panic("implement me")
}

func getRoleBindingsNames(bindings []v1.RoleBinding) []string {
	names := make([]string, len(bindings))
	for i := range bindings {
		names[i] = bindings[i].Name
	}
	return names
}

type twoPagePRTBClient struct {
	fakePRTBClient
	pager *paginationtest.Pager[v3.ProjectRoleTemplateBinding]
}

func (c *twoPagePRTBClient) List(_ string, opts metav1.ListOptions) (*v3.ProjectRoleTemplateBindingList, error) {
	items, next, err := c.pager.Serve(opts)
	if err != nil {
		return nil, err
	}
	return &v3.ProjectRoleTemplateBindingList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
}

type twoPageRoleBindingClient struct {
	fakeRoleBindingClient
	pager *paginationtest.Pager[v1.RoleBinding]
	// deleteErrs maps a role binding name to the error that Delete returns for it.
	deleteErrs map[string]error
}

func (c *twoPageRoleBindingClient) Delete(namespace, name string, opts *metav1.DeleteOptions) error {
	if err, ok := c.deleteErrs[name]; ok {
		return err
	}
	return c.fakeRoleBindingClient.Delete(namespace, name, opts)
}

func (c *twoPageRoleBindingClient) List(_ string, opts metav1.ListOptions) (*v1.RoleBindingList, error) {
	items, next, err := c.pager.Serve(opts)
	if err != nil {
		return nil, err
	}
	return &v1.RoleBindingList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
}

func TestCleanOrphans_Pagination(t *testing.T) {
	t.Parallel()

	groupSubjects := []v1.Subject{{Kind: v1.GroupKind, Name: "Dogs"}}

	validPRTB1 := v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prtb1",
			Namespace: "ns1",
			UID:       "uid-prtb-1",
		},
	}
	validPRTB2 := v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prtb2",
			Namespace: "ns2",
			UID:       "uid-prtb-2",
		},
	}

	rb1 := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-valid-1",
			Namespace: "ns1",
			Labels: map[string]string{
				"ns1_prtb1": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	rb2 := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-orphan-1",
			Namespace: "ns1",
			Labels: map[string]string{
				"ns1_unknown": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	rb3 := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-valid-2",
			Namespace: "ns2",
			Labels: map[string]string{
				"ns2_prtb2": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}
	rb4 := v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-orphan-2",
			Namespace: "ns2",
			Labels: map[string]string{
				"ns2_deleted": auth.PrtbInClusterBindingOwner,
			},
		},
		Subjects: groupSubjects,
	}

	all := []v1.RoleBinding{rb1, rb2, rb3, rb4}
	prtbs := []v3.ProjectRoleTemplateBinding{validPRTB1, validPRTB2}

	tests := []struct {
		name            string
		dryRun          bool
		prtbPageSize    int
		rbPageSize      int
		failPRTBPage    int
		failPRTBErr     error
		failRBPage      int
		deleteErrs      map[string]error
		wantErrIs       []error
		wantPRTBTokens  []string
		wantRBTokens    []string
		wantRemainingRB []string
		wantErr         bool
	}{
		{
			name:            "pages of different sizes clean orphans on every page",
			prtbPageSize:    1,
			rbPageSize:      2,
			wantPRTBTokens:  []string{"", "token-page-1"},
			wantRBTokens:    []string{"", "token-page-1"},
			wantRemainingRB: []string{"rb-valid-1", "rb-valid-2"},
		},
		{
			name:            "one binding per page cleans orphans on every page",
			prtbPageSize:    1,
			rbPageSize:      1,
			wantPRTBTokens:  []string{"", "token-page-1"},
			wantRBTokens:    []string{"", "token-page-1", "token-page-2", "token-page-3"},
			wantRemainingRB: []string{"rb-valid-1", "rb-valid-2"},
		},
		{
			name:            "single page needs one call per list",
			prtbPageSize:    10,
			rbPageSize:      10,
			wantPRTBTokens:  []string{""},
			wantRBTokens:    []string{""},
			wantRemainingRB: []string{"rb-valid-1", "rb-valid-2"},
		},
		{
			name:            "dry run deletes nothing but reads every page",
			dryRun:          true,
			prtbPageSize:    1,
			rbPageSize:      2,
			wantPRTBTokens:  []string{"", "token-page-1"},
			wantRBTokens:    []string{"", "token-page-1"},
			wantRemainingRB: []string{"rb-valid-1", "rb-orphan-1", "rb-valid-2", "rb-orphan-2"},
		},
		{
			name:            "PRTB list error on page 2 deletes nothing",
			prtbPageSize:    1,
			rbPageSize:      2,
			failPRTBPage:    2,
			wantPRTBTokens:  []string{"", "token-page-1"},
			wantRBTokens:    nil,
			wantRemainingRB: []string{"rb-valid-1", "rb-orphan-1", "rb-valid-2", "rb-orphan-2"},
			wantErr:         true,
		},
		{
			name:            "expired PRTB list fails without resuming and deletes nothing",
			prtbPageSize:    1,
			rbPageSize:      2,
			failPRTBPage:    2,
			failPRTBErr:     paginationtest.ExpiredError("new-token"),
			wantPRTBTokens:  []string{"", "token-page-1"},
			wantRBTokens:    nil,
			wantRemainingRB: []string{"rb-valid-1", "rb-orphan-1", "rb-valid-2", "rb-orphan-2"},
			wantErr:         true,
		},
		{
			name:            "role binding list error on page 2 keeps the page 1 cleanup",
			prtbPageSize:    1,
			rbPageSize:      2,
			failRBPage:      2,
			wantPRTBTokens:  []string{"", "token-page-1"},
			wantRBTokens:    []string{"", "token-page-1"},
			wantRemainingRB: []string{"rb-valid-1", "rb-valid-2", "rb-orphan-2"},
			wantErr:         true,
		},
		{
			name:            "role binding list error keeps the delete errors of earlier pages",
			prtbPageSize:    1,
			rbPageSize:      2,
			failRBPage:      2,
			deleteErrs:      map[string]error{"rb-orphan-1": errPgDelete},
			wantPRTBTokens:  []string{"", "token-page-1"},
			wantRBTokens:    []string{"", "token-page-1"},
			wantRemainingRB: []string{"rb-valid-1", "rb-orphan-1", "rb-valid-2", "rb-orphan-2"},
			wantErr:         true,
			wantErrIs:       []error{errPgDelete, paginationtest.ErrListFailed},
		},
		{
			name:            "delete errors on every page are all returned",
			prtbPageSize:    1,
			rbPageSize:      2,
			deleteErrs:      map[string]error{"rb-orphan-1": errPgDelete, "rb-orphan-2": errPgUpdate},
			wantPRTBTokens:  []string{"", "token-page-1"},
			wantRBTokens:    []string{"", "token-page-1"},
			wantRemainingRB: []string{"rb-valid-1", "rb-orphan-1", "rb-valid-2", "rb-orphan-2"},
			wantErr:         true,
			wantErrIs:       []error{errPgDelete, errPgUpdate},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prtbClient := &twoPagePRTBClient{
				fakePRTBClient: fakePRTBClient{bindings: prtbs},
				pager:          paginationtest.NewPager(prtbs, tc.prtbPageSize).FailOnPage(tc.failPRTBPage, tc.failPRTBErr),
			}
			rbClient := &twoPageRoleBindingClient{
				fakeRoleBindingClient: fakeRoleBindingClient{bindings: append([]v1.RoleBinding(nil), all...)},
				pager:                 paginationtest.NewPager(all, tc.rbPageSize).FailOnPage(tc.failRBPage, nil),
				deleteErrs:            tc.deleteErrs,
			}

			cleanup := &orphanBindingsCleanup{
				prtbs:        prtbClient,
				roleBindings: rbClient,
				prtbHashes:   make(map[string]struct{}),
				prtbUIDs:     make(map[string]struct{}),
			}

			err := cleanup.cleanOrphans(tc.dryRun)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tc.wantErrIs {
				if !errors.Is(err, want) {
					t.Errorf("expected error %v to wrap %v", err, want)
				}
			}

			if got := prtbClient.pager.Tokens(); !reflect.DeepEqual(got, tc.wantPRTBTokens) {
				t.Errorf("expected PRTB continue tokens %v, got %v", tc.wantPRTBTokens, got)
			}
			if got := rbClient.pager.Tokens(); !reflect.DeepEqual(got, tc.wantRBTokens) {
				t.Errorf("expected role binding continue tokens %v, got %v", tc.wantRBTokens, got)
			}
			prtbClient.pager.AssertLimit(t)
			rbClient.pager.AssertLimit(t)

			remainingNames := getRoleBindingsNames(rbClient.bindings)
			if !reflect.DeepEqual(remainingNames, tc.wantRemainingRB) {
				t.Errorf("expected remaining role bindings %v, got %v", tc.wantRemainingRB, remainingNames)
			}
		})
	}
}
