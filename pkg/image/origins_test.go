package image

import "testing"

func TestGetAllUniqueImages(t *testing.T) {
	images := [][]string{
		{
			"rancher/hardened-coredns:v1.8.5-build20211119",
			"rancher/hardened-coredns:v1.9.1-build20220318",
			"rancher/hardened-coredns:v1.9.3-build20220613",
		},
		{
			"rancher/hardened-kubernetes:v1.22.10-rke2r2-build20220608",
			"rancher/hardened-kubernetes:v1.22.11-rke2r1-build20220616",
			"rancher/hardened-kubernetes:v1.22.13-rke2r1-build20220817",
		},
	}
	uniqueImages := []string{
		"hardened-coredns",
		"hardened-kubernetes",
	}
	returnedUniqueImages := GetAllUniqueImages(images...)
	for i := range uniqueImages {
		if uniqueImages[i] != returnedUniqueImages[i] {
			t.Errorf("expected: %s, got: %s", uniqueImages, returnedUniqueImages)
			t.Fail()
		}
	}
}

func TestGatherUnknownImages(t *testing.T) {
	fakeImages := []string{
		"fake",
		"phony",
	}
	unknownImages := GatherUnknownImages(fakeImages)
	if len(unknownImages) == 0 {
		t.Errorf("failed to detect unknown images, expected empty string, got: %s", unknownImages)
		t.Fail()
	}
}

func TestRepoFromImage(t *testing.T) {
	image := "rancher/hardened-sriov-network-operator:v1.0.0-build20210429"
	repo := "hardened-sriov-network-operator"

	returnedRepo := repoFromImage(image)
	if repo != returnedRepo {
		t.Errorf("expected: %s, got :%s", repo, returnedRepo)
		t.Fail()
	}

	badImage1 := "hardened-sriov-network-operator:v1.0.0-build20210429"
	badImage2 := "rancher/hardened-sriov-network-operator"

	returnedRepo = repoFromImage(badImage1)
	if returnedRepo != "" {
		t.Errorf("image %s was not handled correctly, expected empty string got %s", badImage1, returnedRepo)
		t.Fail()
	}

	returnedRepo = repoFromImage(badImage2)
	if returnedRepo != "" {
		t.Errorf("image %s was not handled correctly, expected empty string got: %s", badImage2, returnedRepo)
		t.Fail()
	}
}

func TestUniqueTargetImages(t *testing.T) {
	targetImages := []string{
		"rancher/mirrored-calico-operator:v1.28.1",
		"rancher/mirrored-calico-operator:v1.27.1",
		"rancher/mirrored-calico-operator:v1.25.3",
		"rancher/mirrored-calico-pod2daemon-flexvol:v3.17.2",
		"rancher/mirrored-calico-pod2daemon-flexvol:v3.16.5",
		"rancher/mirrored-calico-pod2daemon-flexvol:v3.13.4",
	}

	uniqueImages := []string{
		"mirrored-calico-operator",
		"mirrored-calico-pod2daemon-flexvol",
	}

	returnedUniqueImages := UniqueTargetImages(targetImages)
	for i := range uniqueImages {
		if uniqueImages[i] != returnedUniqueImages[i] {
			t.Fail()
			break
		}
	}

	if t.Failed() {
		t.Errorf("expected: %s, got: %s", uniqueImages, returnedUniqueImages)
	}
}
