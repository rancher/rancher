package image

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pinnedFleet = PinnedChart{Name: testChartName, Version: "111.0.1+up0.12.1"}

func TestCheckPinnedChartsAcceptsThePinThatIsTheNewestOfTheMinor(t *testing.T) {
	root := t.TempDir()
	writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)

	assert.NoError(t, CheckPinnedCharts(root, "2.16.99", []PinnedChart{pinnedFleet}, true))
}

func TestCheckPinnedChartsFindsAPinThatIsNotTheNewestOfTheMinor(t *testing.T) {
	root := t.TempDir()
	writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)
	older := PinnedChart{Name: testChartName, Version: "111.0.0+up0.12.0"}

	err := CheckPinnedCharts(root, "2.16.99", []PinnedChart{older}, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), testChartName)
	assert.Contains(t, err.Error(), "111.0.0+up0.12.0", "the pinned version")
	assert.Contains(t, err.Error(), "111.0.1+up0.12.1", "the version of the list")
}

func TestCheckPinnedChartsFindsAPinThatIsNotInTheIndex(t *testing.T) {
	root := t.TempDir()
	writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)
	missing := PinnedChart{Name: testChartName, Version: "111.0.9+up0.12.9"}

	err := CheckPinnedCharts(root, "2.16.99", []PinnedChart{missing}, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "111.0.9+up0.12.9")
}

func TestCheckPinnedChartsFindsAPinOfAChartWithNoVersionOfTheMinor(t *testing.T) {
	root := t.TempDir()
	writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)

	err := CheckPinnedCharts(root, "2.18.0", []PinnedChart{pinnedFleet}, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), testChartName)
}

func TestCheckPinnedChartsIgnoresAChartThatIsNotInTheRepositories(t *testing.T) {
	root := t.TempDir()
	writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)
	other := PinnedChart{Name: "rancher-webhook", Version: "111.0.1+up0.12.1"}

	assert.NoError(t, CheckPinnedCharts(root, "2.16.99", []PinnedChart{other}, true))
}

func TestCheckPinnedChartsOnlyWarnsWhenItIsNotStrict(t *testing.T) {
	root := t.TempDir()
	writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)
	older := PinnedChart{Name: testChartName, Version: "111.0.0+up0.12.0"}

	assert.NoError(t, CheckPinnedCharts(root, "2.16.99", []PinnedChart{older}, false))
}

func TestCheckPinnedChartsReportsEveryMismatch(t *testing.T) {
	root := t.TempDir()
	writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)
	pins := []PinnedChart{
		{Name: testChartName, Version: "111.0.0+up0.12.0"},
		{Name: testChartName, Version: "110.0.0+up0.11.0"},
	}

	err := CheckPinnedCharts(root, "2.16.99", pins, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "111.0.0+up0.12.0")
	assert.Contains(t, err.Error(), "110.0.0+up0.11.0")
}

func TestCheckPinnedChartsLooksInEveryRepository(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeTestIndexOfVersions(t, first, versionsOfSeveralMinors[:1])
	writeTestIndexOfVersions(t, second, versionsOfSeveralMinors)

	assert.NoError(t, CheckPinnedCharts(first+","+second, "2.16.99", []PinnedChart{pinnedFleet}, true))
}
