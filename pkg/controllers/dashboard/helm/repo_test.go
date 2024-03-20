package helm

import (
	"testing"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldRefresh(t *testing.T) {
	tests := []struct {
		name           string
		spec           *catalog.RepoSpec
		status         *catalog.RepoStatus
		expectedResult bool
	}{
		{
			"http repo - spec equals status",
			&catalog.RepoSpec{
				URL: "https://example.com",
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now(),
				},
			},
			false,
		},
		{
			"http repo - url changed",
			&catalog.RepoSpec{
				URL: "https://changed-url.com",
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now(),
				},
			},
			true,
		},
		{
			"http repo - missing indexConfigMap",
			&catalog.RepoSpec{
				URL: "https://example.com",
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "",
				DownloadTime: metav1.Time{
					Time: time.Now(),
				},
			},
			true,
		},
		{
			"http repo - download not so long ago",
			&catalog.RepoSpec{
				URL: "https://example.com",
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now().Add(-1 * time.Minute),
				},
			},
			false,
		},
		{
			"http repo - download to long ago",
			&catalog.RepoSpec{
				URL: "https://example.com",
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now().Add(-7 * time.Hour),
				},
			},
			true,
		},
		{
			"http repo - force update",
			&catalog.RepoSpec{
				URL: "https://example.com",
				ForceUpdate: &metav1.Time{
					Time: time.Now(),
				},
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now().Add(-1 * time.Minute),
				},
			},
			true,
		},
		{
			"http repo - force update older than download time",
			&catalog.RepoSpec{
				URL: "https://example.com",
				ForceUpdate: &metav1.Time{
					Time: time.Now().Add(-2 * time.Minute),
				},
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now().Add(-1 * time.Minute),
				},
			},
			false,
		},
		{
			"http repo - force update in the future",
			&catalog.RepoSpec{
				URL: "https://example.com",
				ForceUpdate: &metav1.Time{
					Time: time.Now().Add(2 * time.Minute),
				},
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now(),
				},
			},
			false,
		},
		{
			"git repo - spec equals status",
			&catalog.RepoSpec{
				GitBranch: "master",
				GitRepo:   "git.example.com",
			},
			&catalog.RepoStatus{
				Branch:             "master",
				URL:                "git.example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now(),
				},
			},
			false,
		},
		{
			"git repo - branch changed",
			&catalog.RepoSpec{
				GitBranch: "main",
				GitRepo:   "git.example.com",
			},
			&catalog.RepoStatus{
				Branch:             "master",
				URL:                "git.example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now(),
				},
			},
			true,
		},
		{
			"git repo - repo changed",
			&catalog.RepoSpec{
				GitBranch: "master",
				GitRepo:   "newgit.example.com",
			},
			&catalog.RepoStatus{
				Branch:             "master",
				URL:                "git.example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now(),
				},
			},
			true,
		},
		{
			"http repo - spec equals status, but unnecessary git branch in spec",
			&catalog.RepoSpec{
				URL:       "https://example.com",
				GitBranch: "master",
			},
			&catalog.RepoStatus{
				URL:                "https://example.com",
				IndexConfigMapName: "configmap",
				DownloadTime: metav1.Time{
					Time: time.Now(),
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldRefresh(tt.spec, tt.status)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
